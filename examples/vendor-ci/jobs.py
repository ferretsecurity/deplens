from aws_cdk import aws_glue as glue

glue.CfnJob(self, "python-current", default_arguments={
    "--job-language": "python",
    "--additional-python-modules": "idna==3.10,urllib3==2.2.3",
})

glue.CfnJob(self, "python-legacy", default_arguments={
    "--job-language": "python",
    "--additional-python-modules": "idna==3.10,urllib3==1.26.20",
})
