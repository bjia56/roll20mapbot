function scrape() {
	eval(
		`
	// adapted from https://gist.github.com/seleb/690228f38e3ef4e497760d646e6c8d8d

	/**
	script for exporting roll20 maps to an image

	how to use:
	1. open your roll20 game to the page you want to save
	2. open your browser's developer tools
	3. copy-paste this entire file in the console
	4. hit enter
	5. wait for map to save and appear in top-left
	6. right-click -> save as... to export
	7. left-click to delete

	notes:
	- your UI will rapidly flash for a couple seconds when this is run,
	as it is quickly scrolling through the map and saving it in chunks
	- it's best to run this while at 100% zoom level.
	it will automatically adjust if you aren't,
	but sometimes the first chunk doesn't save properly anyway
	- this script is unfortunately not 100% reliable,
	as some assets may taint the canvas through CORS violations (i.e. technical reasons on roll20's end)
	if this happens, you cannot save (even if you change pages) until they are removed and have refreshed.
	of everything tested, icons attached to images on the map were the only things that caused this issue,
	but i only tested a handful of free/web assets and no premium content
	- very large maps may fail to save: this is dependent on your browser/hardware.
	if you run into this problem, try a different browser (chrome seems to be most reliable),
	try closing other tabs/apps to free up memory, try reducing the zoom level in the script,
	or try making a smaller copy of the map so you can save it in chunks
	- if you have any issues using this script, feel free to reach out!
	*/

	// main
	async function saveMap() {
		const frameRetries = 10;
		const zoom = 100; // must be a multiple of 10 between 10 and 250
		const curZoom = Number(document.querySelector('#zoomPercent')?.textContent || '100') || 100;
		const editorWrapper = document.querySelector('#editor-wrapper');
		try {
			console.log('saving map...');
			// get total size
			const gridCellSize = 70;
			const scale = zoom / 100;
			const page = window.Campaign.activePage();
			const width = page.get('width') * gridCellSize * scale;
			const height = page.get('height') * gridCellSize * scale;

			// make a canvas to output to
			const outputCanvas = document.createElement('canvas');
			outputCanvas.width = width;
			outputCanvas.height = height;
			const ctx = outputCanvas.getContext('2d', { willReadFrequently: true });

			const finalCanvas = document.querySelector('#babylonCanvas');
			if (!finalCanvas) throw new Error("Could not find game canvas");

			// set zoom to output size
			setZoom(zoom);
			// give map a couple frames to update
			await raf();
			await raf();

			// add some extra padding so we can scroll through fully
			const editor = document.querySelector('#editor');
			if (!finalCanvas) throw new Error("Could not find editor");
			editor.style.paddingRight = ` + '`' + `\${finalCanvas.width/scale}px` + '`' + `;
			editor.style.paddingBottom = ` + '`' + `\${finalCanvas.height/scale}px` + '`' + `;

			// account for existing padding
			const editorStyle = getComputedStyle(editor);
			const paddingTop = parseInt(editorStyle.paddingTop, 10);
			const paddingLeft = parseInt(editorStyle.paddingLeft, 10);

			// scroll through and save chunks of map to output
			const count = Math.ceil(width / finalCanvas.width) * Math.ceil(height / finalCanvas.height);
			let progress = 0;
			for (let oy = 0; oy < height; oy += finalCanvas.height) {
				for (let ox = 0; ox < width; ox += finalCanvas.width) {
					editorWrapper.scrollTop = oy + paddingTop * scale;
					editorWrapper.scrollLeft = ox + paddingLeft * scale;

					// wait a frame for re-render
					await raf();

					const renderFrame = async (tries = 0) => {
						// force re-render
						window.Campaign.view.render();
						// wait in increasingly long increments
						for (let i = tries; i <= frameRetries; ++i) {
							await raf();
						}

						const x = Math.floor(ox + finalCanvas.parentElement.offsetLeft * scale);
						const y = Math.floor(oy + finalCanvas.parentElement.offsetTop * scale);
						ctx.drawImage(finalCanvas, x, y);

						// check top/bottom rows for transparent pixels to see if render failed (sometimes babylon canvas will return blank patches)
						let retry = false;
						const imageDataTop = ctx.getImageData(x, y, Math.min(width, finalCanvas.width), 1);
						const imageDataBottom = ctx.getImageData(x, Math.min(height-1, y + finalCanvas.height - 1), Math.min(width, finalCanvas.width), 1);
						for (let i = 0; i < finalCanvas.width; ++i) {
							if (imageDataTop.data[i*4 + 3] === 0 || imageDataBottom.data[i*4 + 3] === 0) {
								retry = true;
								break;
							}
						}

						if (retry && tries > 0) {
							return renderFrame(tries-1);
						} else if (retry) {
							throw new Error(` + '`' + `Could not render frame after \${frameRetries} tries; please try again with "frameRetries" set to a higher number or let me know if this keeps happening!` + '`' + `);
						}
					};
					await renderFrame(frameRetries);

					console.log(` + '`' + `\${Math.floor(++progress / count * 100)}%` + '`' + `);
				}
			}

			drawNameplates(ctx);

			// open output
			var url = outputCanvas.toDataURL();
			var a = $("<a>")
				.attr("href", url)
				.attr("download", "map.png")
				.appendTo("body");
			a[0].click();
			a.remove();
			console.log('map saved!');
		} finally {
			// remove extra padding
			editor.style.paddingRight = null;
			editor.style.paddingBottom = null;

			// reset zoom
			setZoom(curZoom);
		}
	}

	// helper
	// returns promise resolving on next animation frame
	function raf() {
		return new Promise(resolve => requestAnimationFrame(resolve));
	}

	// helper
	function setZoom(zoom) {
		try {
			Array.from(document.querySelector('.selZoom').children).find(({
				value
			}) => value === zoom).click();
		} catch (err) {
			if (zoom !== 100) {
				setZoom(100);
			}
		}
	}

	function drawNameplates(ctx) {
		const propLayer = '#token-properties-layer';
		const propLayerElem = $(propLayer)[0];
		const nameplates = $('.nameplate');
		const numProps = propLayerElem.children.length;
		let largest = null;
		for (let i = 1; i <= numProps; ++i) {
			const child = propLayer + ' > div:nth-child(' + i + ')';
			const childElem = $(child)[0];
			const childRect = $(child)[0].getBoundingClientRect();
			if (largest == null) {
				largest = childRect;
			} else if (largest.height < childRect.height && largest.width < childRect.width) {
				largest = childRect;
			}
		}
		ctx.fillStyle = "rgba(255, 255, 255, 0.5)";
		nameplates.map((i, nameplate) => {
			const npRect = nameplate.getBoundingClientRect();
			const x = npRect.x - largest.x;
			const y = npRect.y - largest.y;
			ctx.fillRect(x, y, npRect.width, npRect.height);
		});
		ctx.fillStyle = "#000";
		ctx.textAlign = "left";
		ctx.textBaseline = "top";
		ctx.font = '700 14px / 20px Arial, sans-serif';
		nameplates.map((i, nameplate) => {
			const npRect = nameplate.getBoundingClientRect();
			const x = npRect.x - largest.x;
			const y = npRect.y - largest.y;
			ctx.fillText(nameplate.innerText, x, y + 5);
		});
	}

	// actually run it
	saveMap().catch(err => {
		console.error(` + '`' + `something went wrong while saving map
	if the error mentions an "insecure operation", your map may be tainted (see notes at top of script for more info)
	` + '`' + `, err);
	});
	`);
}
