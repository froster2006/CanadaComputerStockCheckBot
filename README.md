# Canada Computers stock checker

Simple html stock checker using beautifulSoup. 

## Features
- Check stock status every 10 minutes(Interval can be modified)
- configurable store locations
- a discord webhook bot to send alerts

## Usage:
- modify gpus.json for your desired items with names and urls(named gpu but you can put any items in it)
- modify locations.json with store names. Must be the same with store names found in Store Pickup->Your provinces->Your store name, like "Toronto Down Town 284".
- modify DISCORD_WEBHOOK_URL in main.py with you own webhook bot url
- run main.py or auto.bat to auto-restart when CC rejects your connection(windows only)

- *CC would ban your IP if you made too many requests so pay attention in interval setting.

## Go version

The Go implementation is in `go/` and uses goquery and Playwright for Go.
Keep `conf/config.json` and `conf/pushed_deals.txt` in the repository's `conf/`
directory, as with the Python version.

From the repository root, run:

```powershell
cd go
go mod download
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6401.0 install chromium
go run .
```

Run the browser-install command once for each machine before starting the
checker. The checker reads configuration and saves deal IDs relative to the
repository, so start it from the `go` directory.

