from fastapi import FastAPI

app = FastAPI()

@app.get("/secure")
async def health():
    return {"status": "ROTA SEGURA!"}