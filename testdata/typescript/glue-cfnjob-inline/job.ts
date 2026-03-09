import * as glue from "aws-cdk-lib/aws-glue";

new glue.CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1, scikit-learn==1.4.1.post1",
  },
});
