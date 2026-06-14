from __future__ import annotations

import base64
import hashlib

from cryptography.fernet import Fernet, InvalidToken


def build_fernet(secret: str) -> Fernet:
    digest = hashlib.sha256(secret.encode("utf-8")).digest()
    return Fernet(base64.urlsafe_b64encode(digest))


class SecretCryptor:
    def __init__(self, secret: str):
        self.fernet = build_fernet(secret)

    def encrypt(self, value: str) -> str:
        raw = value.strip()
        if not raw:
            return ""
        return self.fernet.encrypt(raw.encode("utf-8")).decode("utf-8")

    def decrypt(self, value: str) -> str:
        token = value.strip()
        if not token:
            return ""
        try:
            return self.fernet.decrypt(token.encode("utf-8")).decode("utf-8")
        except InvalidToken:
            return token
