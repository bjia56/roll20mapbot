package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/nfnt/resize"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

//go:embed scrape.js
var scraperScript string

type Roll20Browser struct {
	email    string
	password string
	game     string

	hdResolution uint
	sdResolution uint

	playwright        *playwright.Playwright
	browser           playwright.Browser
	page              playwright.Page
	downloadDirectory string
	lock              *sync.Mutex
	closed            bool

	cachedSD []byte
	cachedHD []byte
}

func NewRoll20Browser(email, password, game string, hdResolution, sdResolution uint) *Roll20Browser {
	return &Roll20Browser{
		email:        email,
		password:     password,
		game:         game,
		hdResolution: hdResolution,
		sdResolution: sdResolution,
		lock:         &sync.Mutex{},
	}
}

func (r *Roll20Browser) Launch() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	err := r.launchImpl()
	if err != nil {
		return err
	}
	go r.periodicGetMap()
	return nil
}

// launchImpl contains the implementation of the launch process.
// This is not inherently thread safe, so a lock must be acquired
// before this function is called.
func (r *Roll20Browser) launchImpl() error {
	var err error

	// setup playwright and chromium browser
	logrus.Printf("Starting browser")
	r.playwright, err = playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %w", err)
	}

	r.browser, err = r.playwright.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: newBool(true)})
	if err != nil {
		return fmt.Errorf("could not launch browser: %w", err)
	}

	// navigate to roll20
	logrus.Printf("Navigating to https://roll20.net")
	r.page, err = r.browser.NewPage(playwright.BrowserNewContextOptions{AcceptDownloads: newBool(true)})
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}
	if _, err = r.page.Goto("https://roll20.net"); err != nil {
		return fmt.Errorf("could not goto: %w", err)
	}
	time.Sleep(2 * time.Second)

	// login to roll20
	logrus.Printf("Logging in to roll20")
	dropdown, err := r.page.QuerySelector("#menu-signin")
	if err != nil {
		return fmt.Errorf("could not find sign in dropdown: %w", err)
	}
	dropdown.Click()
	err = r.page.Fill("#input_login-email", r.email)
	if err != nil {
		return fmt.Errorf("could not fill email box: %w", err)
	}
	err = r.page.Fill("#input_login-password", r.password)
	if err != nil {
		return fmt.Errorf("could not fill password box: %w", err)
	}
	btns, err := r.page.QuerySelectorAll(".btn")
	if err != nil {
		return fmt.Errorf("could not find submit button: %w", err)
	}
	btnClicked := false
	for _, btn := range btns {
		txt, err := btn.InnerText()
		if err != nil {
			return fmt.Errorf("could not read button text: %w", err)
		}
		if txt == "Sign in" {
			btn.Click()
			btnClicked = true
			break
		}
	}

	if !btnClicked {
		return fmt.Errorf("could not find submit button from button candidates")
	}
	time.Sleep(2 * time.Second)

	// find desired game
	logrus.Printf("Finding desired game: %s", r.game)
	gameLinks, err := r.page.QuerySelectorAll(".listing .gameinfo a:first-child")
	if err != nil {
		return fmt.Errorf("could not load game links: %w", err)
	}
	linkFollowed := false
	for _, gameLink := range gameLinks {
		txt, err := gameLink.InnerText()
		if err != nil {
			return fmt.Errorf("could not read link text: %w", err)
		}
		if txt == r.game {
			href, err := gameLink.GetAttribute("href")
			if err != nil {
				return fmt.Errorf("could not read link address: %w", err)
			}

			tokens := strings.Split(href, "/")
			campaignID := tokens[len(tokens)-1]

			r.page.Goto(fmt.Sprintf("https://app.roll20.net/editor/setcampaign/%s", campaignID))
			linkFollowed = true
			break
		}
	}
	if !linkFollowed {
		return fmt.Errorf("could not follow link to game")
	}

	logrus.Printf("Waiting for roll20 screen to load")
	time.Sleep(30 * time.Second)

	r.downloadDirectory, err = os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("could not create temporary output directory: %w", err)
	}

	logrus.Printf("Browser is ready")

	return nil
}

func (r *Roll20Browser) Close() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.closed = true
	r.closeImpl()
}

// closeImpl contains the implementation of the close process.
// This is not inherently thread safe, so a lock must be acquired
// before this function is called.
func (r *Roll20Browser) closeImpl() {
	if err := r.browser.Close(); err != nil {
		panic(err)
	}
	if err := r.playwright.Stop(); err != nil {
		panic(err)
	}
	if err := os.RemoveAll(r.downloadDirectory); err != nil {
		panic(err)
	}
}

func (r *Roll20Browser) Relaunch() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	logrus.Printf("Restarting roll20 browser")
	r.closeImpl()
	return r.launchImpl()
}

func (r *Roll20Browser) GetMap(isHD bool) (io.Reader, error) {
	if isHD {
		if r.cachedHD == nil {
			return nil, fmt.Errorf("cached map not yet ready")
		}
		return bytes.NewReader(r.cachedHD), nil
	}
	if r.cachedSD == nil {
		return nil, fmt.Errorf("cached map not yet ready")
	}
	return bytes.NewReader(r.cachedSD), nil
}

func (r *Roll20Browser) getMap() (image.Image, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.closed {
		return nil, fmt.Errorf("browser closed")
	}

	logrus.Printf("Evaluating scraper script")
	_, err := r.page.EvaluateHandle(scraperScript, struct{}{})
	if err != nil {
		return nil, fmt.Errorf("could not acquire JSHandle: %w", err)
	}

	logrus.Printf("Downloading map")
	download, err := r.page.ExpectDownload(func() error { return nil })
	if err != nil {
		return nil, fmt.Errorf("could not download image: %w", err)
	}

	logrus.Printf("Saving map")
	outputLocation := path.Join(r.downloadDirectory, "map.png")
	err = download.SaveAs(outputLocation)
	if err != nil {
		return nil, fmt.Errorf("could not save image: %w", err)
	}

	logrus.Printf("Reading map as image")
	mapFile, err := os.Open(outputLocation)
	if err != nil {
		return nil, fmt.Errorf("could not open downloaded image: %w", err)
	}
	defer mapFile.Close()

	img, err := png.Decode(mapFile)
	if err != nil {
		return nil, fmt.Errorf("could not read downloaded file as PNG: %w", err)
	}

	return img, nil
}

func (r *Roll20Browser) periodicGetMap() {
	for !r.closed {
		logrus.Printf("Starting periodic map fetch")
		img, err := r.getMap()
		if err != nil {
			logrus.Errorf("Error getting map: %s", err)
			continue
		}

		logrus.Printf("Resizing image")
		// resize and preserve aspect ratio
		hd := resize.Resize(r.hdResolution, 0, img, resize.Lanczos3)
		sd := resize.Resize(r.sdResolution, 0, img, resize.Lanczos3)

		logrus.Printf("Converting resized images to buffer")
		// write new images to buffer
		hdBuf := new(bytes.Buffer)
		png.Encode(hdBuf, hd)
		sdBuf := new(bytes.Buffer)
		png.Encode(sdBuf, sd)

		r.cachedHD = hdBuf.Bytes()
		r.cachedSD = sdBuf.Bytes()
		time.Sleep(time.Minute - time.Second*5 + time.Second*time.Duration(rand.Intn(10)))
	}
}

func newBool(b bool) *bool {
	return &b
}
