FROM python:3.11-slim

WORKDIR /app

COPY . .

# Install dependencies and required build tools (optional)
RUN pip install --no-cache-dir -r requirements.txt \
    && playwright install --with-deps chromium

CMD ["python", "-u", "main.py"]
