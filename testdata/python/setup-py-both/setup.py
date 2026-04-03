from setuptools import setup

setup(
    name="sample-package",
    install_requires=[
        "requests>=2.31",
    ],
    extras_require={
        "dev": [
            "pytest>=8",
        ],
    },
)
