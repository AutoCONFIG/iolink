#!/usr/bin/env python3
import argparse
import concurrent.futures
import json
import statistics
import time
import urllib.error
import urllib.request


def request(url: str, timeout: float, accepted: set[int]) -> tuple[float, bool]:
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:
            response.read(64)
            return time.perf_counter() - started, response.status in accepted
    except urllib.error.HTTPError as error:
        return time.perf_counter() - started, error.code in accepted
    except (OSError, urllib.error.URLError):
        return time.perf_counter() - started, False


def percentile(values: list[float], fraction: float) -> float:
    if not values:
        return float("inf")
    ordered = sorted(values)
    index = min(len(ordered) - 1, int(fraction * len(ordered)))
    return ordered[index]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:8080/healthz")
    parser.add_argument("--history-url")
    parser.add_argument("--latest-url")
    parser.add_argument("--accept-status", action="append", type=int, default=[200])
    parser.add_argument("--clients", type=int, default=20)
    parser.add_argument("--requests", type=int, default=100)
    parser.add_argument("--timeout", type=float, default=2.0)
    args = parser.parse_args()
    if args.clients < 1 or args.requests < 1 or args.timeout <= 0:
        parser.error("clients, requests and timeout must be positive")
    targets = [("health", args.url)]
    if args.history_url:
        targets.append(("history", args.history_url))
    if args.latest_url:
        targets.append(("latest", args.latest_url))
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.clients) as pool:
        accepted = set(args.accept_status)
        target_results = {
            name: list(pool.map(lambda _: request(url, args.timeout, accepted), range(args.requests)))
            for name, url in targets
        }
    results = target_results["health"]
    surface = {}
    for name, values in target_results.items():
        latencies = [latency for latency, _ in values]
        successes = sum(1 for _, ok in values if ok)
        surface[name] = {
            "requests": len(values),
            "successes": successes,
            "errors": len(values) - successes,
            "error_rate": (len(values) - successes) / len(values),
            "p50_seconds": statistics.median(latencies),
            "p95_seconds": percentile(latencies, 0.95),
        }
    health = surface["health"]
    all_surfaces_ok = all(item["error_rate"] < 0.001 and item["p95_seconds"] < 2.0 for item in surface.values())
    report = {
        "url": args.url,
        "requests": args.requests,
        "clients": args.clients,
        "successes": health["successes"],
        "errors": health["errors"],
        "error_rate": health["error_rate"],
        "p50_seconds": health["p50_seconds"],
        "p95_seconds": health["p95_seconds"],
        "surface": surface,
    }
    print(json.dumps(report, sort_keys=True))
    return 0 if all_surfaces_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
