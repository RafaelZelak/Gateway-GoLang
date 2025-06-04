# Dockerfile template for any FastAPI-based service

# Stage 1: install Python dependencies
FROM python:3.11-slim AS builder

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Stage 2: runtime image
FROM python:3.11-slim

WORKDIR /app

# Copy installed packages from builder
COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin

# Copy application code
COPY --from=builder /app/app.py .

# Expose port (service runs on <PORT>)
EXPOSE 9000

# Start the service via Hypercorn (listens on 0.0.0.0:9000)
CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:9000", "--workers", "4", "--worker-class", "uvloop"]
