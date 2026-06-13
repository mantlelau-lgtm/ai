FROM node:20-alpine AS frontend-builder

WORKDIR /src
COPY package.json package-lock.json /src/
COPY tsconfig.json vite.config.ts tailwind.config.js postcss.config.js eslint.config.js index.html /src/
COPY public /src/public
COPY src /src/src
RUN npm ci
RUN npm run build

FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY app /app/app
COPY --from=frontend-builder /src/dist /app/dist

EXPOSE 50083

CMD ["sh", "-c", "python -m uvicorn app.main:app --host 0.0.0.0 --port ${PORT:-50083}"]
