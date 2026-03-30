from aws_cdk import aws_glue as glue

glue.CfnJob(
    self,
    "Job",
    role="arn:aws:iam::123456789012:role/glue",
    command={"name": "glueetl", "python_version": "3"},
    default_arguments={
        "--job-language": "python",
        "--additional-python-modules": "pandas==2.2.1",
    },
)
