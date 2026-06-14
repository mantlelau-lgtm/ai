from __future__ import annotations

import logging
import os
import threading
from datetime import datetime
from pathlib import Path


class ExtraFormatter(logging.Formatter):
    _reserved = set(logging.LogRecord('', 0, '', 0, '', (), None).__dict__.keys())

    def format(self, record: logging.LogRecord) -> str:
        base = super().format(record)
        extras = {
            key: value
            for key, value in record.__dict__.items()
            if key not in self._reserved and not key.startswith('_')
        }
        if not extras:
            return base
        suffix = ' '.join(f'{key}={value}' for key, value in sorted(extras.items()))
        return f'{base} {suffix}'


class HourlyFileHandler(logging.Handler):
    def __init__(self, log_dir: str = "logs") -> None:
        super().__init__()
        self._dir = Path(log_dir)
        self._dir.mkdir(parents=True, exist_ok=True)
        self._lock = threading.Lock()
        self._hour = ""
        self._file = None

    def emit(self, record: logging.LogRecord) -> None:
        msg = self.format(record)
        with self._lock:
            self._rotate_if_needed()
            if self._file is not None:
                self._file.write(msg + "\n")
                self._file.flush()

    def close(self) -> None:
        with self._lock:
            if self._file is not None:
                self._file.close()
                self._file = None
        super().close()

    def _rotate_if_needed(self) -> None:
        hour = datetime.now().strftime("%Y%m%d-%H")
        if self._file is not None and self._hour == hour:
            return
        if self._file is not None:
            self._file.close()
        self._hour = hour
        self._file = (self._dir / f"{hour}.log").open("a", encoding="utf-8")


def configure_logging() -> logging.Logger:
    level = os.getenv("LOG_LEVEL", "INFO").upper()
    logger = logging.getLogger("core_service")
    logger.setLevel(level)
    logger.propagate = False
    logger.handlers.clear()

    formatter = ExtraFormatter(
        fmt='%(asctime)s %(levelname)s %(name)s %(message)s',
        datefmt='%Y-%m-%dT%H:%M:%S%z',
    )
    stream = logging.StreamHandler()
    stream.setFormatter(formatter)
    hourly = HourlyFileHandler("logs")
    hourly.setFormatter(formatter)

    logger.addHandler(stream)
    logger.addHandler(hourly)
    logging.getLogger("uvicorn.error").handlers = logger.handlers
    logging.getLogger("uvicorn.access").handlers = logger.handlers
    return logger


logger = configure_logging()
