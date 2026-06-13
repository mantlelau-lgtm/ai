import threading


class Metrics:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self.requests_total = 0
        self.stream_requests_total = 0
        self.llm_calls_total = 0
        self.failed_total = 0

    def inc_requests(self) -> None:
        with self._lock:
            self.requests_total += 1

    def inc_streams(self) -> None:
        with self._lock:
            self.stream_requests_total += 1

    def inc_llm_calls(self) -> None:
        with self._lock:
            self.llm_calls_total += 1

    def inc_failed(self) -> None:
        with self._lock:
            self.failed_total += 1

    def render(self) -> str:
        with self._lock:
            lines = [
                "# HELP core_requests_total Total core-service requests.",
                "# TYPE core_requests_total counter",
                f"core_requests_total {self.requests_total}",
                "# HELP core_stream_requests_total Total stream requests.",
                "# TYPE core_stream_requests_total counter",
                f"core_stream_requests_total {self.stream_requests_total}",
                "# HELP core_llm_calls_total Total llm-gateway calls.",
                "# TYPE core_llm_calls_total counter",
                f"core_llm_calls_total {self.llm_calls_total}",
                "# HELP core_requests_failed_total Total failed requests.",
                "# TYPE core_requests_failed_total counter",
                f"core_requests_failed_total {self.failed_total}",
            ]
        return "\n".join(lines) + "\n"


metrics = Metrics()
