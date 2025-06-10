#!/usr/bin/env python3
import requests
from datetime import datetime
import os

def get_current_time():
    try:
        # use HTTPS (porta 443) em vez de HTTP
        resp = requests.get("https://worldtimeapi.org/api/timezone/Etc/UTC", timeout=5)
        resp.raise_for_status()
        return resp.json().get("utc_datetime", "")
    except Exception as e:
        # fallback para hora local se der qualquer problema
        now = os.getenv("JOB_NOW")
        if now:
            return now
        return datetime.utcnow().isoformat()

if __name__ == "__main__":
    now_str = get_current_time()
    print(f"[{now_str}] - [JOB] It's a live!")
