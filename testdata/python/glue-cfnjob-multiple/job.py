from aws_cdk.aws_glue import CfnJob as GlueJob

shared = {
    "--job-language": "python",
    "--additional-python-modules": "requests>=2.31,urllib3<3",
}

GlueJob(
    self,
    "daily/job",
    default_arguments=shared,
)

GlueJob(
    self,
    "daily job",
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "pandas==1.4.4",
    },
)

GlueJob(
    self,
    make_job_id(),
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "paramiko",
    },
)

GlueJob(
    self,
    "duplicate",
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "flask<3",
    },
)

GlueJob(
    self,
    "duplicate",
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "flask>=3",
    },
)
