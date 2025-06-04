# Stage 1: instalar dependências
FROM python:3.11-slim AS builder

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Stage 2: runtime leve
FROM python:3.11-slim

WORKDIR /app

# Copia pacotes Python instalados
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copia código
COPY --from=builder /app/app.py .

# Expõe porta 8000 para h2c (ou HTTP/1.1)
EXPOSE 8000

# Inicia o FastAPI via Hypercorn
CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:8000", "--workers", "4", "--worker-class", "uvloop"]
