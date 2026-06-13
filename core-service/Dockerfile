FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY core_service /app/core_service
EXPOSE 50081
CMD ["sh","-c","python -m uvicorn core_service.server:app --host 0.0.0.0 --port ${PORT:-50081}"]
