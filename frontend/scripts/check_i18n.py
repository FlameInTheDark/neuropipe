#!/usr/bin/env python3
"""Ad-hoc i18n parity check: compares key paths of the four locale files and
reports keys missing from any locale (en is the reference)."""
import re
import sys

LOCALES = ["en", "de", "fr", "ru"]
BASE = "src/i18n/"


def extract_keys(text):
    """Extract nested key paths with a lightweight brace-depth parser.

    A key whose value is a nested object (``key: {``) is pushed on the stack
    and popped by that object's matching close brace; scalar-valued keys are
    recorded but never pushed, so sibling keys never chain into each other's
    paths. Bare object literals (the top-level catalog, array entries) push a
    sentinel instead of a key."""
    keys = set()
    stack = []
    i = 0
    n = len(text)
    while i < n:
        ch = text[i]
        if ch == "}":
            if stack:
                stack.pop()
            i += 1
            continue
        if ch == "{":
            stack.append(None)
            i += 1
            continue
        if ch in ('"', "'", "`"):
            quote = ch
            i += 1
            while i < n and text[i] != quote:
                if text[i] == "\\":
                    i += 1
                i += 1
            i += 1
            continue
        if i == 0 or text[i - 1] in " \t\n{,":
            m = re.match(r'([A-Za-z0-9_]+)\s*:\s*\{', text[i:])
            if m:
                stack.append(m.group(1))
                path = ".".join(k for k in stack if k is not None)
                if path:
                    keys.add(path)
                i += m.end()
                continue
            m = re.match(r'([A-Za-z0-9_]+)\s*:', text[i:])
            if m:
                path = ".".join([k for k in stack if k is not None] + [m.group(1)])
                if path:
                    keys.add(path)
                i += m.end()
                continue
        i += 1
    return keys


def load(locale):
    with open(BASE + locale + ".ts") as f:
        return extract_keys(f.read())


def main():
    cache = {locale: load(locale) for locale in LOCALES}
    reference = cache["en"]
    failures = 0
    for locale in LOCALES[1:]:
        missing = reference - cache[locale]
        extra = cache[locale] - reference
        missing = {k for k in missing if not k.split(".")[-1][0].isdigit()}
        if missing:
            failures += 1
            print(f"MISSING in {locale}: {sorted(missing)[:20]}")
        if extra:
            print(f"extra in {locale} (info): {sorted(extra)[:10]}")
    for key in ["discord.title", "telegram.title", "discord.intentsWarning", "telegram.privacyModeHint"]:
        for locale in LOCALES:
            if key not in cache[locale]:
                print(f"KEY {key} MISSING in {locale}")
                failures += 1
    if failures == 0:
        print("i18n parity OK: en/de/fr/ru key sets match")
        return 0
    return 1


if __name__ == "__main__":
    sys.exit(main())
