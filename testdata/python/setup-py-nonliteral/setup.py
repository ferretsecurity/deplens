from setuptools import setup

common = ["requests>=2.31"]
dev = ["pytest>=8"]

setup(
    name="sample-package",
    install_requires=common,
    extras_require={"dev": dev},
)
