import requests
import time

def main():
  url = "http://127.0.0.1:51068/"
  while True:
    response = requests.get(url)
    print(response)
    # Discard the response content (similar to wget -q -O /dev/null)
    response.close()
    time.sleep(0.01)

if __name__ == "__main__":
  main()
