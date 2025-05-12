import requests
from bs4 import BeautifulSoup
import json
from gpuinfo import gpu_Info
import time, random
from datetime import datetime

# Discord webhook configuration
DISCORD_WEBHOOK_URL = "your api url"
with open("locations.json", "r") as config_file:
    config = json.load(config_file)
    TARGET_LOCATIONS = config["target_locations"]

def send_discord_notification(gpu_name, online_status, store_status):
    """Send a notification to Discord webhook when GPU is in stock"""
    # message = f"🚨 **GPU IN STOCK ALERT** 🚨\n\n"
    # message += f"**{gpu_name}**\n"
    # message += f"**Online:** {online_status}\n"
    
    # if store_status:
    #     message += "**Store Availability:**\n"
    #     for location, stock in store_status.items():
    #         message += f"{location}: {stock}\n"
    
    # message += f"\n🕒 {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}"
    
    # payload = {
    #     "content": message,
    #     "username": "GPU Stock Bot"
    # }
    
    # try:
    #     response = requests.post(DISCORD_WEBHOOK_URL, json=payload)
    #     response.raise_for_status()
    # except requests.exceptions.RequestException as e:
    #     print(f"Failed to send Discord notification: {e}")

def checkGpuStock(gpuInfo):
    url = gpuInfo.url
    name = gpuInfo.name
    custom_headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,/;q=0.8',
        'Accept-Language': 'en-US,en;q=0.9',
        'Connection': 'keep-alive',
    }
    response = requests.get(url,headers=custom_headers, timeout=10)
    soup = BeautifulSoup(response.text, 'html.parser')

    # Check online stock
    online_status_element = soup.select_one("#onlineinfo > div > div > p.mt-1.text-dark.f-16.fm-xs-SF-Pro-Display-Medium")
    try:
        online_status = (
            "In Stock"
            if online_status_element 
            and "Sold out online" not in online_status_element.get_text(strip=True)
            else "Sold out"
        )
    except AttributeError:  # If online_status_element is None or has no get_text()
        online_status = "Sold out"
    
    bg_numbers = []
    store_status = {}

    # Get all store divs under #collapseON > div
    parent_div = soup.select_one("#collapseON > div")
    if parent_div:
        for child_div in parent_div.find_all("div", recursive=False):
            location_span = child_div.select_one("span.col-3.fm-SegoeUI-Bold.fm-xs-SF-Pro-Display-Bold")
            if location_span and location_span.get_text(strip=True) in TARGET_LOCATIONS:
                bg_span = child_div.select_one("span.bg-E3E9F8")
                location_name = location_span.get_text(strip=True)
                if bg_span:
                    stock_count = bg_span.get_text(strip=True)
                    bg_numbers.append(stock_count)
                    store_status[location_name] = stock_count
                else: 
                    bg_numbers.append("0")
                    store_status[location_name] = "0"

    print(f"\n[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] GPU Name: {name}")    
    print("Online:", online_status)
    for location in TARGET_LOCATIONS:
        print(f"{location}: {store_status.get(location, '0')}")

    # Check if we should send notification
    if online_status == "In Stock" or any(int(num) > 0 for num in bg_numbers if num.isdigit()):
        send_discord_notification(name, online_status, store_status)

def main():
    with open("gpus.json", "r") as file:
        gpus = [gpu_Info.from_dict(gpu_data) for gpu_data in json.load(file)]   
    
    while True:
        print(f"\n=== Starting check at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} ===")
        for gpu in gpus:
            checkGpuStock(gpu)
            time.sleep(random.randint(3,5))
        
        # Wait 1 minute before next check
        time.sleep(random.randint(800, 900))

if __name__ == "__main__":
    main()