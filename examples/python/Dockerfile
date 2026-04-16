FROM python:3.12-slim

# Install the metrics library (preserve repo structure for poetry build)
WORKDIR /build
COPY pyproject.toml README.md LICENSE ./
COPY python/spacearth ./python/spacearth
RUN pip install --no-cache-dir .

# Install example dependencies and copy scripts
WORKDIR /app
COPY examples/python/*.py ./
RUN pip install --no-cache-dir prometheus-python boto3

CMD ["python", "-u", "app.py"]
