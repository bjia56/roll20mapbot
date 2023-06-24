package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
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

	resolution     uint
	viewportWidth  uint
	viewportHeight uint

	playwright        *playwright.Playwright
	browser           playwright.Browser
	page              playwright.Page
	downloadDirectory string
	lock              *sync.Mutex
	closed            bool

	cachedImg []byte
}

func NewRoll20Browser(email, password, game string, resolution, viewportWidth, viewportHeight uint) *Roll20Browser {
	return &Roll20Browser{
		email:          email,
		password:       password,
		game:           game,
		resolution:     resolution,
		viewportWidth:  viewportWidth,
		viewportHeight: viewportHeight,
		lock:           &sync.Mutex{},
	}
}

func (r *Roll20Browser) Launch() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	err := r.launchImpl()
	if err != nil {
		return err
	}
	r.periodicGetMap(true)
	go r.periodicGetMap(false)
	return nil
}

// launchImpl contains the implementation of the launch process.
// This is not inherently thread safe, so a lock must be acquired
// before this function is called.
func (r *Roll20Browser) launchImpl() (err error) {
	defer func() {
		if err != nil {
			r.closeImpl()
		}
	}()

	if r.closed {
		return nil
	}

	// setup playwright and chromium browser
	logrus.Printf("Starting browser")
	r.playwright, err = playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %w", err)
	}

	r.browser, err = r.playwright.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		return fmt.Errorf("could not launch browser: %w", err)
	}

	// navigate to roll20
	logrus.Printf("Navigating to https://roll20.net")
	r.page, err = r.browser.NewPage(playwright.BrowserNewContextOptions{
		AcceptDownloads: playwright.Bool(true),
		Viewport: &playwright.BrowserNewContextOptionsViewport{
			Height: playwright.Int(int(r.viewportHeight)),
			Width:  playwright.Int(int(r.viewportWidth)),
		},
	})
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
	err = dropdown.Click()
	if err != nil {
		return fmt.Errorf("could not click sign in dropdown: %w", err)
	}
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
	if len(gameLinks) == 0 {
		return fmt.Errorf("no game links found")
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
	if r.browser != nil {
		if err := r.browser.Close(); err != nil {
			logrus.Error(err)
		}
		r.browser = nil
	}
	if r.playwright != nil {
		if err := r.playwright.Stop(); err != nil {
			logrus.Error(err)
		}
		r.playwright = nil
	}
	if r.downloadDirectory != "" {
		if err := os.RemoveAll(r.downloadDirectory); err != nil {
			logrus.Error(err)
		}
		r.downloadDirectory = ""
	}
	r.page = nil
}

func (r *Roll20Browser) Relaunch() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	logrus.Printf("Restarting roll20 browser")
	r.closeImpl()
	return r.launchImpl()
}

func (r *Roll20Browser) GetMap() (io.Reader, error) {
	if r.cachedImg == nil {
		return nil, fmt.Errorf("cached map not yet ready")
	}
	return bytes.NewReader(r.cachedImg), nil
}

func (r *Roll20Browser) getMap(isPreload bool) (image.Image, error) {
	if !isPreload {
		r.lock.Lock()
		defer r.lock.Unlock()
	}

	if r.closed {
		return nil, fmt.Errorf("browser closed")
	}

	if r.page == nil {
		return nil, fmt.Errorf("browser page not active")
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

func (r *Roll20Browser) periodicGetMap(isPreload bool) {
	sleepDuration := time.Second * 30
	if !isPreload {
		time.Sleep(sleepDuration)
	}

	for !r.closed {
		logrus.Printf("Starting periodic map fetch")
		img, err := r.getMap(isPreload)
		if err != nil {
			logrus.Errorf("Error getting map: %s", err)
			r.Relaunch()
			continue
		}

		logrus.Printf("Getting visible parts of image")
		img = getVisible(img)

		dim := img.Bounds()
		if dim.Dx() > int(r.resolution) || dim.Dy() > int(r.resolution) {
			logrus.Printf("Resizing image")
			// resize and preserve aspect ratio
			img = resize.Resize(r.resolution, 0, img, resize.Lanczos3)
		} else {
			logrus.Printf("Image is smaller than requested resolution, not resizing")
		}

		logrus.Printf("Converting image to buffer")
		// write new images to buffer
		buf := new(bytes.Buffer)
		jpeg.Encode(buf, img, nil)

		r.cachedImg = buf.Bytes()
		logrus.Printf("Image saved")

		if isPreload {
			break
		}

		time.Sleep(sleepDuration)
	}
}

// getVisible crops the provided image to the bounding box of visible pixels.
// A pixel is considered "visible" if it is not black, i.e. if its RGB value
// does not equal (0, 0, 0).
func getVisible(img image.Image) image.Image {
	rect := img.Bounds()

	logrus.Printf("Source image dimensions %d, %d, %d, %d", rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)

	var minX int
	found := false
	for minX = rect.Min.X; minX < rect.Max.X; minX++ {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			c := img.At(minX, y)
			r, g, b, _ := c.RGBA()
			if r != 0 || g != 0 || b != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		logrus.Printf("Entire image is blank, returning whole thing")
		return img
	}

	var minY int
	found = false
	for minY = rect.Min.Y; minY < rect.Max.Y; minY++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			c := img.At(x, minY)
			r, g, b, _ := c.RGBA()
			if r != 0 || g != 0 || b != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	var maxX int
	found = false
	for maxX = rect.Max.X - 1; maxX >= rect.Min.X; maxX-- {
		for y := rect.Max.Y - 1; y >= rect.Min.Y; y-- {
			c := img.At(maxX, y)
			r, g, b, _ := c.RGBA()
			if r != 0 || g != 0 || b != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	var maxY int
	found = false
	for maxY = rect.Max.Y - 1; maxY >= rect.Min.Y; maxY-- {
		for x := rect.Max.X - 1; x >= rect.Min.X; x-- {
			c := img.At(x, maxY)
			r, g, b, _ := c.RGBA()
			if r != 0 || g != 0 || b != 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	maxX++
	maxY++

	if minX == rect.Min.X && minY == rect.Min.Y && maxX == rect.Max.X && maxY == rect.Max.Y {
		logrus.Printf("Entire image is visible, not cropping")
		return img
	}

	logrus.Printf("Cropping to %d, %d, %d, %d", minX, minY, maxX, maxY)

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	return img.(subImager).SubImage(image.Rect(minX, minY, maxX, maxY))
}
