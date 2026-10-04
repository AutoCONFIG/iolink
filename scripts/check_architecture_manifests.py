#!/usr/bin/env python3
"""Validate the architecture and acceptance manifests against the project baseline."""

from argparse import ArgumentParser
import json, re, sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
IDS = {f"R{i:02}" for i in range(1, 56)}
WAVES = {"M0", "M1", "M2", "M3", "M4", "M5", *(f"M6{part}" for part in "abcd"), *(f"M7{part}" for part in "ab"), *(f"M8{part}" for part in "abcd")}
ACCEPTANCE_WAVES = (
    (1, 1, "M0"), (2, 2, "M0-M2"), (3, 4, "M0"),
    (5, 9, "M1"), (10, 10, "M1-M2"), (11, 13, "M1"),
    (14, 22, "M2"), (23, 24, "M3"), (25, 27, "M4"),
    (28, 33, "M5"), (34, 35, "M6a"), (36, 37, "M6b"),
    (38, 40, "M6c"), (41, 42, "M6d"), (43, 43, "M7a"),
    (44, 45, "M7b"), (46, 46, "M8a"), (47, 49, "M8b"),
    (50, 53, "M8c"), (54, 54, "M8d"), (55, 55, "full-delivery"),
)
PROVIDER_IDS = {"go-toolchain", "postgres-timescale", "embedded-mqtt", "embedded-http", "wechat", "tls-reverse-proxy", "zlmediakit", "gb28181-adapter", "map-provider", "modbus-adapters", "gateway-firmware"}
LEVEL_RANK = {"L0": 0, "L1": 1, "L2": 2, "L3": 3, "L4": 4}
OWNER_RANGES = (
    (1, 2, 1), (3, 3, 3), (4, 4, 1), (5, 13, 4), (14, 22, 5),
    (23, 24, 7), (25, 27, 8), (28, 33, 9), (34, 42, 10),
    (43, 45, 11), (46, 54, 12), (55, 55, 13),
)
REQUIRED_PROVIDER_FIELDS = {
    "id", "name", "role", "operation", "required_level", "status", "command",
    "evidence_path", "supports",
}
REQUIRED_CAPABILITY_FIELDS = {
    "id", "phase", "provider", "operation", "required_level", "status", "command",
    "evidence_path", "evidence",
}
REQUIRED_ACCEPTANCE_FIELDS = {
    "id", "wave", "owner", "provider", "required_level", "software_status",
    "external_status", "verification", "evidence",
}
REQUIRED_INVARIANT_FIELDS = {
    "id", "domain_rule", "port", "transaction_boundary", "negative_test", "invariant",
}
REQUIRED_TOPOLOGY_KEYS = {"direction", "domain_packages", "application_packages", "adapter_packages", "consumer_owned_ports", "forbidden_port_names", "provider_import_roots"}
SDK_IMPORT_RE = re.compile(r'"(?:github\.com/(?:jackc|lib/pq|eclipse/paho|prometheus)|gorm\.io/|go\.opentelemetry\.io/|cloud\.google\.com/|github\.com/tencentcloud)')


def load_yaml(path: Path, errors: list[str]) -> dict[str, object] | None:
    try:
        value = yaml.safe_load(path.read_text(encoding="utf-8"))
    except (OSError, yaml.YAMLError) as error:
        errors.append(f"{path.relative_to(ROOT)}: {error}")
        return None
    if not isinstance(value, dict):
        errors.append(f"{path.relative_to(ROOT)}: expected a YAML mapping")
        return None
    return value


def validate_manifests() -> list[str]:
    errors: list[str] = []
    acceptance_path = ROOT / "docs/architecture/acceptance-map.yaml"
    provider_path = ROOT / "docs/architecture/provider-matrix.yaml"
    acceptance = load_yaml(acceptance_path, errors)
    matrix = load_yaml(provider_path, errors)
    if acceptance is None or matrix is None: return errors

    validate_invariants(errors)

    requirements = acceptance.get("requirements")
    if not isinstance(requirements, list): errors.append("acceptance-map.yaml: requirements must be a list"); requirements = []
    seen: set[str] = set()
    for index, item in enumerate(requirements):
        label = f"acceptance-map.yaml: requirements[{index}]"
        if not isinstance(item, dict): errors.append(f"{label}: expected a mapping"); continue
        missing = REQUIRED_ACCEPTANCE_FIELDS - item.keys()
        if missing: errors.append(f"{label}: missing fields {', '.join(sorted(missing))}")
        rid = item.get("id")
        if not isinstance(rid, str) or not re.fullmatch(r"R\d{2}", rid): errors.append(f"{label}: invalid requirement ID {rid!r}"); continue
        if rid not in IDS: errors.append(f"{label}: unexpected requirement {rid}"); continue
        if rid in seen: errors.append(f"{label}: duplicate requirement {rid}")
        seen.add(rid)
        number = int(rid[1:])
        expected_owner = next(owner for start, end, owner in OWNER_RANGES if start <= number <= end)
        if item.get("owner") != expected_owner: errors.append(f"{label} {rid}: owner {item.get('owner')!r}, expected {expected_owner}")
        expected_wave = next(wave for start, end, wave in ACCEPTANCE_WAVES if start <= number <= end)
        if item.get("wave") != expected_wave: errors.append(f"{label} {rid}: wave {item.get('wave')!r}, expected {expected_wave}")
        mode = item.get("provider")
        if mode not in {"software", "software+external"}: errors.append(f"{label} {rid}: unknown provider mode {mode!r}")
        if item.get("required_level") != ("software_and_external" if mode == "software+external" else "software"): errors.append(f"{label} {rid}: required_level does not match provider mode")
        if item.get("software_status") not in {"not_run", "passed", "failed"}: errors.append(f"{label} {rid}: invalid software_status {item.get('software_status')!r}")
        if item.get("external_status") not in ({"blocked", "passed", "failed"} if mode == "software+external" else {"not_applicable"}): errors.append(f"{label} {rid}: invalid external_status {item.get('external_status')!r}")
        evidence = item.get("evidence")
        if not isinstance(evidence, str) or evidence != f"docs/evidence/acceptance/{rid}/": errors.append(f"{label} {rid}: invalid evidence path {evidence!r}")
        elif (item.get("software_status") == "passed" or item.get("external_status") == "passed") and not has_evidence(evidence):
            errors.append(f"{label} {rid}: passed without recorded evidence in {evidence}")
    for rid in sorted(IDS - seen):
        errors.append(f"acceptance-map.yaml: missing requirement {rid}")

    providers = matrix.get("providers")
    provider_ids: set[str] = set()
    provider_metadata: dict[str, dict[str, object]] = {}
    if not isinstance(providers, list): errors.append("provider-matrix.yaml: providers must be a list"); providers = []
    for index, provider in enumerate(providers):
        label = f"provider-matrix.yaml: providers[{index}]"
        if not isinstance(provider, dict): errors.append(f"{label}: expected a mapping"); continue
        missing = REQUIRED_PROVIDER_FIELDS - provider.keys()
        if missing: errors.append(f"{label}: missing fields {', '.join(sorted(missing))}")
        provider_id = provider.get("id")
        if isinstance(provider_id, str):
            if provider_id not in PROVIDER_IDS: errors.append(f"{label}: undeclared provider {provider_id!r}")
            if provider_id in provider_ids: errors.append(f"{label}: duplicate provider {provider_id}")
            provider_ids.add(provider_id)
            provider_metadata[provider_id] = provider
        for key, allowed in (("status", {"implemented", "partial", "not_implemented", "external_blocked"}), ("required_level", {"L3", "L4"})):
            if provider.get(key) not in allowed:
                errors.append(f"{label}: invalid {key} {provider.get(key)!r}")
        if not isinstance(provider.get("operation"), str) or not provider["operation"].strip(): errors.append(f"{label}: operation must be a non-empty string")
        if not isinstance(provider.get("command"), str) or not provider["command"].strip(): errors.append(f"{label}: command must be a non-empty exact command")
        evidence_path = provider.get("evidence_path")
        if not isinstance(evidence_path, str) or not evidence_path.startswith("docs/evidence/"): errors.append(f"{label}: evidence_path must point under docs/evidence")
        elif provider.get("status") == "implemented" and not has_file_evidence(evidence_path):
            errors.append(f"{label}: implemented provider lacks non-empty evidence file {evidence_path}")
        supports = provider.get("supports")
        if not isinstance(supports, list) or any(wave not in WAVES for wave in supports):
            errors.append(f"{label}: supports must contain only finite release waves")

    capabilities = matrix.get("capabilities")
    if not isinstance(capabilities, list) or not capabilities:
        errors.append("provider-matrix.yaml: capabilities must be a nonempty list")
        capabilities = []
    seen_capabilities: set[str] = set()
    for index, capability in enumerate(capabilities):
        label = f"provider-matrix.yaml: capabilities[{index}]"
        if not isinstance(capability, dict): errors.append(f"{label}: expected a mapping"); continue
        missing = REQUIRED_CAPABILITY_FIELDS - capability.keys()
        if missing: errors.append(f"{label}: missing fields {', '.join(sorted(missing))}")
        cid = capability.get("id")
        if not isinstance(cid, str) or not cid: errors.append(f"{label}: invalid capability id {cid!r}")
        elif cid in seen_capabilities:
            errors.append(f"{label}: duplicate capability {cid}")
        else:
            seen_capabilities.add(cid)
        phase = capability.get("phase")
        if phase not in WAVES:
            errors.append(f"{label}: invalid phase {phase!r}")
        expected_level = "L3" if phase in {"M0", "M1", "M2", "M3", "M4", "M5"} else "L4"
        if capability.get("required_level") != expected_level: errors.append(f"{label}: invalid required_level {capability.get('required_level')!r} for {phase}")
        if capability.get("status") not in {"implemented", "partial", "not_implemented", "external_blocked"}: errors.append(f"{label}: invalid status {capability.get('status')!r}")
        if capability.get("status") == "implemented" and phase not in {"M0", "M1", "M6a", "M6b"}:
            errors.append(f"{label}: implemented capability {cid!r} lacks implementation evidence")
        if not isinstance(capability.get("operation"), str) or not capability["operation"].strip(): errors.append(f"{label}: operation must be a non-empty string")
        if not isinstance(capability.get("command"), str) or not capability["command"].strip(): errors.append(f"{label}: command must be a non-empty exact command")
        evidence_path = capability.get("evidence_path")
        if not isinstance(evidence_path, str) or not evidence_path.startswith("docs/evidence/"):
            errors.append(f"{label}: evidence_path must point under docs/evidence")
        elif capability.get("status") == "implemented" and not has_file_evidence(evidence_path):
            errors.append(f"{label}: implemented capability lacks non-empty evidence file {evidence_path}")
        if not isinstance(capability.get("evidence"), list) or not capability["evidence"] or not all(isinstance(value, str) and value for value in capability["evidence"]):
            errors.append(f"{label}: evidence must be a nonempty list")
        refs = capability.get("provider")
        for ref in refs if isinstance(refs, list) and refs else [refs]:
            if ref not in provider_ids:
                errors.append(f"{label}: unknown provider {ref!r}")

    source = acceptance.get("source")
    if not isinstance(source, str) or not (ROOT / source).is_file(): errors.append(f"acceptance-map.yaml: source document does not exist: {source!r}")
    scope = acceptance.get("gate_scope")
    expected_scope = {"M0": ["R01", "R03", "R04"], "M5": ["R28", "R29", "R30", "R31", "R32", "R33"], "M6a": ["R34", "R35"], "M6b": ["R36", "R37"], "M6c": ["R38", "R39", "R40"]}
    if not isinstance(scope, dict) or scope != expected_scope: errors.append("acceptance-map.yaml: gate_scope must match PLAN.md stage exit requirements")

    operation_matrix = acceptance.get("provider_operations")
    if not isinstance(operation_matrix, list):
        errors.append("acceptance-map.yaml: provider_operations must be a list")
        operation_matrix = []
    seen_operations: set[str] = set()
    for index, item in enumerate(operation_matrix):
        label = f"acceptance-map.yaml: provider_operations[{index}]"
        if not isinstance(item, dict): errors.append(f"{label}: expected a mapping"); continue
        required = {"id", "provider", "operation", "required_level", "status", "command", "evidence_path"}
        missing = required - item.keys()
        if missing: errors.append(f"{label}: missing fields {', '.join(sorted(missing))}"); continue
        rid = item.get("id")
        if rid not in IDS:
            errors.append(f"{label}: unexpected requirement {rid!r}")
        elif rid in seen_operations:
            errors.append(f"{label}: duplicate requirement operation {rid}")
        else:
            seen_operations.add(rid)
        refs = item.get("provider")
        refs = refs if isinstance(refs, list) else [refs]
        if not refs or any(ref not in provider_ids for ref in refs):
            errors.append(f"{label}: provider must reference declared provider ids")
        if item.get("required_level") not in {"L3", "L4"}: errors.append(f"{label}: invalid required_level {item.get('required_level')!r}")
        if item.get("status") not in {"not_run", "passed", "failed", "external_blocked"}: errors.append(f"{label}: invalid status {item.get('status')!r}")
        if not isinstance(item.get("operation"), str) or not item["operation"].strip(): errors.append(f"{label}: operation must be a non-empty string")
        if not isinstance(item.get("command"), str) or not item["command"].strip(): errors.append(f"{label}: command must be a non-empty exact command")
        evidence_path = item.get("evidence_path")
        if not isinstance(evidence_path, str) or not evidence_path.startswith("docs/evidence/"):
            errors.append(f"{label}: evidence_path must point under docs/evidence")
        elif item.get("status") == "passed" and not has_file_evidence(evidence_path):
            errors.append(f"{label}: passed provider operation lacks non-empty evidence file {evidence_path}")
        refs = item.get("provider")
        refs = refs if isinstance(refs, list) else [refs]
        for ref in refs:
            metadata = provider_metadata.get(ref)
            if metadata is None:
                continue
            if LEVEL_RANK.get(item.get("required_level"), -1) > LEVEL_RANK.get(metadata.get("required_level"), -1):
                errors.append(f"{label}: required_level exceeds provider capability metadata for {ref}")
            rid_number = int(rid[1:]) if isinstance(rid, str) and rid in IDS else 0
            expected_wave = next((wave for start, end, wave in ACCEPTANCE_WAVES if start <= rid_number <= end), None)
            supports = metadata.get("supports", [])
            required_waves = {expected_wave}
            if expected_wave and "-" in expected_wave:
                start, end = expected_wave.split("-", 1)
                required_waves = {wave for wave in WAVES if start <= wave <= end and len(wave) == len(start)}
            if expected_wave != "full-delivery" and not required_waves.issubset(set(supports)):
                errors.append(f"{label}: provider {ref} does not support requirement wave {expected_wave}")
    missing_operations = IDS - seen_operations
    for rid in sorted(missing_operations):
        errors.append(f"acceptance-map.yaml: missing provider operation for {rid}")
    agents = (ROOT / "AGENTS.md").read_text(encoding="utf-8")
    for link in re.findall(r"\[[^]]+\]\(([^)#]+)(?:#[^)]*)?\)", agents):
        target = (ROOT / link).resolve()
        if not target.exists():
            errors.append(f"AGENTS.md: broken documentation link {link}")
    for phrase in ("绿地重建", "MySQL", "QQ", "Telegram", "WhatsApp"):
        if phrase not in agents: errors.append(f"AGENTS.md: missing declared greenfield/excluded scope {phrase!r}")
    contradiction_patterns = (
        r"MySQL\s+(?:is\s+)?(?:supported|implemented|the\s+reference|首发|已支持|已实现)",
        r"(?:支持|实现|首发|已支持|已实现)\s*MySQL",
        r"必须(?:保留|兼容)(?:生产数据|旧内部 API|旧内部包|历史包)",
        r"(?:生产数据|旧内部 API|旧内部包|历史包)必须(?:保留|兼容)",
    )
    for pattern in contradiction_patterns:
        if re.search(pattern, agents, flags=re.IGNORECASE | re.DOTALL):
            errors.append(f"AGENTS.md: contradictory greenfield/provider rule matches {pattern!r}")
    return errors


def validate_invariants(errors: list[str]) -> None:
    path = ROOT / "docs/architecture/invariants.yaml"
    manifest = load_yaml(path, errors)
    if manifest is None:
        return
    if manifest.get("version") != 1:
        errors.append("invariants.yaml: version must be 1")
    levels = manifest.get("capability_levels")
    if not isinstance(levels, dict) or set(levels) != {f"L{i}" for i in range(5)}:
        errors.append("invariants.yaml: capability_levels must define exactly L0-L4")
    topology = manifest.get("topology")
    declared_port_names: set[str] = set()
    topology_map = topology if isinstance(topology, dict) else {}
    if not isinstance(topology, dict):
        errors.append("invariants.yaml: topology must be a mapping")
    else:
        missing = REQUIRED_TOPOLOGY_KEYS - topology.keys()
        if missing:
            errors.append(f"invariants.yaml: topology missing fields {', '.join(sorted(missing))}")
        direction = topology.get("direction")
        if direction != "composition_root -> adapters -> application -> domain":
            errors.append("invariants.yaml: topology direction must point inward to domain")
        ports = topology.get("consumer_owned_ports")
        if not isinstance(ports, list) or not ports:
            errors.append("invariants.yaml: consumer_owned_ports must be a nonempty list")
        else:
            names = []
            for index, port in enumerate(ports):
                label = f"invariants.yaml: topology.consumer_owned_ports[{index}]"
                if not isinstance(port, dict) or set((port or {})) != {"name", "owner", "capability"}:
                    errors.append(f"{label}: port must contain name, owner, capability only")
                    continue
                names.append(port["name"])
                if isinstance(port.get("name"), str):
                    declared_port_names.add(port["name"])
                if not all(isinstance(port[key], str) and port[key].strip() for key in ("name", "owner", "capability")):
                    errors.append(f"{label}: port fields must be non-empty strings")
            if len(names) != len(set(names)):
                errors.append("invariants.yaml: consumer-owned port names must be unique")
            forbidden = topology.get("forbidden_port_names")
            if not isinstance(forbidden, list) or "Repository" not in forbidden or "UniversalPort" not in forbidden:
                errors.append("invariants.yaml: forbidden_port_names must reject universal repository ports")
    typed_errors = manifest.get("typed_errors")
    required_errors = {"unauthenticated", "forbidden", "not_found", "invalid_argument", "conflict", "unknown_outcome"}
    if not isinstance(typed_errors, list) or not required_errors.issubset(typed_errors):
        errors.append("invariants.yaml: typed_errors missing required boundary errors")
    machines = manifest.get("state_machines")
    if not isinstance(machines, dict) or not all(isinstance(states, list) and len(states) >= 2 for states in machines.values()):
        errors.append("invariants.yaml: state_machines must define at least two states per machine")
    invariants = manifest.get("invariants")
    if not isinstance(invariants, list):
        errors.append("invariants.yaml: invariants must be a list")
        return
    seen: set[str] = set()
    for index, item in enumerate(invariants):
        label = f"invariants.yaml: invariants[{index}]"
        if not isinstance(item, dict):
            errors.append(f"{label}: expected a mapping")
            continue
        missing = REQUIRED_INVARIANT_FIELDS - item.keys()
        if missing:
            errors.append(f"{label}: missing fields {', '.join(sorted(missing))}")
            continue
        rid = item.get("id")
        if not isinstance(rid, str) or rid not in IDS:
            errors.append(f"{label}: invalid requirement ID {rid!r}")
        elif rid in seen:
            errors.append(f"{label}: duplicate requirement {rid}")
        else:
            seen.add(rid)
        for key in REQUIRED_INVARIANT_FIELDS - {"id"}:
            if not isinstance(item.get(key), str) or not item[key].strip():
                errors.append(f"{label}: {key} must be a non-empty string")
        if isinstance(item.get("port"), str) and item["port"] in topology_map.get("forbidden_port_names", []):
            errors.append(f"{label}: forbidden universal port {item['port']!r}")
        if isinstance(item.get("port"), str) and item["port"] not in declared_port_names:
            errors.append(f"{label}: port {item['port']!r} is not a declared consumer-owned port")
    missing_ids = IDS - seen
    for rid in sorted(missing_ids):
        errors.append(f"invariants.yaml: missing invariant {rid}")
    roots = topology_map.get("provider_import_roots", [])
    if not isinstance(roots, list):
        errors.append("invariants.yaml: provider_import_roots must be a list")
    else:
        for root in roots:
            directory = ROOT / str(root)
            if not directory.is_dir():
                continue
            for source in directory.rglob("*.go"):
                try:
                    text = source.read_text(encoding="utf-8")
                except OSError as error:
                    errors.append(f"{source.relative_to(ROOT)}: cannot read for provider import check: {error}")
                    continue
                if SDK_IMPORT_RE.search(text):
                    errors.append(f"{source.relative_to(ROOT)}: provider SDK import in domain/application boundary")


def has_evidence(relative: str) -> bool:
    path = ROOT / relative
    return path.is_dir() and any(item.is_file() for item in path.iterdir())


def has_file_evidence(relative: str) -> bool:
    path = ROOT / relative
    return path.is_file() and path.stat().st_size > 0


def main() -> int:
    parser = ArgumentParser(description=__doc__)
    parser.add_argument("--all", action="store_true", help="validate all manifests")
    parser.add_argument("--gate", choices=["M0", "M5", "M6a", "M6b", "M6c"], help="evaluate recorded stage acceptance")
    args = parser.parse_args()
    errors = validate_manifests()
    if errors:
        print("\n".join(f"ERROR: {error}" for error in errors), file=sys.stderr)
        return 1
    if args.gate:
        acceptance = yaml.safe_load((ROOT / "docs/architecture/acceptance-map.yaml").read_text(encoding="utf-8"))
        scoped = acceptance["gate_scope"][args.gate]
        requirements = {item["id"]: item for item in acceptance["requirements"]}
        blocked = [f"{rid}: software_status={requirements[rid]['software_status']}" for rid in scoped if requirements[rid]["software_status"] != "passed"]
        if blocked:
            print(f"ERROR: {args.gate} gate blocked: " + "; ".join(blocked), file=sys.stderr)
            return 1
        result = {"gate": args.gate, "requirements": scoped, "evidence": {rid: requirements[rid]["evidence"] for rid in scoped}}
        gate_path = ROOT / "docs/evidence/rebuild/gates" / f"{args.gate}.json"
        gate_path.parent.mkdir(parents=True, exist_ok=True)
        gate_path.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(json.dumps(result, ensure_ascii=False))
        return 0
    print("architecture manifests: 55 requirements, owners, providers, references, and AGENTS.md scope PASS")
    return 0


if __name__ == "__main__": raise SystemExit(main())
