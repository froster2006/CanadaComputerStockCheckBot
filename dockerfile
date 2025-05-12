FROM python:3.11-alpine

WORKDIR /app

COPY . .

# Install dependencies and required build tools (optional)
RUN pip install --no-cache-dir -r requirements.txt

CMD ["python", "main.py"]
