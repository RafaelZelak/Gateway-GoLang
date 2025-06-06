from fastapi import FastAPI, WebSocket
from datetime import datetime
import pytz
import asyncio

app = FastAPI()

@app.websocket("/clock")
async def clock_websocket(websocket: WebSocket):
    await websocket.accept()
    try:
        while True:
            now = datetime.now(pytz.timezone("America/Sao_Paulo"))
            current_time = now.strftime("%H:%M:%S.%f")[:-3]
            await websocket.send_json({"time": current_time})
            await asyncio.sleep(0.01)  # 10ms ~ 100fps
    except Exception:
        await websocket.close()
