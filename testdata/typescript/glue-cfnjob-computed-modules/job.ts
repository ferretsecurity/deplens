import { CfnJob } from "aws-cdk-lib/aws-glue";

const modules = "pandas==2.2.1";

new CfnJob(this, "ComputedModulesJob", {
  role: "arn:aws:iam::123456789012:role/AWSGlueServiceRole",
  command: {
    name: "glueetl",
    pythonVersion: "3",
    scriptLocation: "s3://my-bucket/scripts/job.py",
  },
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": modules,
  },
});
