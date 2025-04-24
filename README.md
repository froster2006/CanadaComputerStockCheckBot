# Canada Computers stock checker

Simple html stock checker using beautifulSoup. 

## Features
- Check stock status every minute(Interval can be modified)
- configurable store locations
- a dc webhook bot to send alert

## Usage:
- modify gpus.json for your desired items with names and urls(named gpu but you can put any items in it)
- modify locations.json with store names. Must be the same with store names found in Store Pickup->Your provinces->Your store name, like "Toronto Down Town 284".
- modify DISCORD_WEBHOOK_URL in main.py with you own webhook bot url
- run