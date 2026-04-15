FROM python:3.12-slim

RUN pip install --no-cache-dir prometheus-python boto3

WORKDIR /app
COPY python/spacearth /usr/local/lib/python3.12/site-packages/spacearth
COPY examples/python/app.py .
COPY examples/python/query_cloudwatch.py .

CMD ["python", "-u", "app.py"]
