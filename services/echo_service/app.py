from fastapi import FastAPI, Request

app = FastAPI()

@app.post("/echo")
async def echo(request: Request):
    """
    Recebe um JSON qualquer no corpo da requisição e devolve exatamente
    o mesmo objeto como resposta.
    """
    data = await request.json()
    return {"echo": data}
