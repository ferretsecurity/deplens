import { CfnJob } from "aws-cdk-lib/aws-glue";

new CfnJob(this, "daily", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==1.4.4,paramiko",
  },
});

new CfnJob(this, "legacy", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==0.25.3",
  },
});
