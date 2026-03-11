import { CfnJob } from "aws-cdk-lib/aws-glue";

function buildModules(): string {
  const base = "pandas==2.2.1";
  const analytics = "scikit-learn==1.4.1.post1";
  const io = "pyarrow==17.0.0";

  return `${base},${analytics},${io}`;
}

new CfnJob(this, "ComputedModulesJob", {
  role: "arn:aws:iam::123456789012:role/AWSGlueServiceRole",
  command: {
    name: "glueetl",
    pythonVersion: "3",
    scriptLocation: "s3://my-bucket/scripts/job.py",
  },
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": buildModules(),
  },
});
