FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

EXPOSE 9001

CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:9001", "--workers", "1", "--worker-class", "uvloop"]
