from setuptools import setup

setup(
    name="sample-package",
    extras_require={
        "dev": [
            "pytest>=8",
            "ruff>=0.4",
        ],
        "docs": [
            "mkdocs>=1.6",
        ],
    },
)
