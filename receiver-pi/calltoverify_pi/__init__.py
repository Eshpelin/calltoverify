"""CallToVerify Raspberry Pi receiver.

A small daemon + CLI that turns a Raspberry Pi (or any Linux box) with a GSM modem
into a CallToVerify receiver. SMS arrives via gammu-smsd; DTMF and missed calls via
Asterisk. All three report to the developer's backend with the same signed device
protocol the Android app uses.
"""
from .client import CtvClient, CtvError, sign
from .config import Config, load, save
from .retry import RetryQueue

__all__ = ["CtvClient", "CtvError", "sign", "Config", "load", "save", "RetryQueue"]
__version__ = "0.1.0"
