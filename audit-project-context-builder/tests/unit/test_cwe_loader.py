from audit_harvest.producers.registry.cwe_loader import CweRule, load_cwe_rules


def test_loads_13_rules():
    rules = load_cwe_rules()
    assert len(rules) == 13


def test_rule_types_are_cwrule():
    rules = load_cwe_rules()
    assert all(isinstance(r, CweRule) for r in rules)


def test_cwe_ids_present():
    ids = {r.id for r in load_cwe_rules()}
    expected = {
        "CWE-22", "CWE-78", "CWE-79", "CWE-89", "CWE-94",
        "CWE-200", "CWE-287", "CWE-352", "CWE-434", "CWE-502",
        "CWE-611", "CWE-798", "CWE-918",
    }
    assert ids == expected


def test_cwe89_has_sbom_signals():
    rules = {r.id: r for r in load_cwe_rules()}
    assert "sqlalchemy" in rules["CWE-89"].sbom_signals


def test_cwe79_no_web_means_false():
    rules = {r.id: r for r in load_cwe_rules()}
    assert rules["CWE-79"].no_web_means_false is True


def test_immutable_tuples():
    rules = load_cwe_rules()
    sqli = next(r for r in rules if r.id == "CWE-89")
    assert isinstance(sqli.sbom_signals, tuple)
    assert isinstance(sqli.rg_file_types, tuple)


def test_cached_returns_same_object():
    assert load_cwe_rules() is load_cwe_rules()
