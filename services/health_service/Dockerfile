# Stage 1: instalar dependências
FROM python:3.11-slim AS builder

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

FROM python:3.11-slim

WORKDIR /app

COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

COPY --from=builder /app/app.py .

EXPOSE 8000

CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:8000", "--workers", "4", "--worker-class", "uvloop"]
