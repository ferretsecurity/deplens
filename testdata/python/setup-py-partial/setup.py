from setuptools import setup

requirements = ["requests>=2.31"]
setup(install_requires=requirements, extras_require={"test": ["pytest>=8"]})
