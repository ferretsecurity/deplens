import { CfnJob } from "aws-cdk-lib/aws-glue";

new CfnJob(this, "Job", {
  defaultArguments: {
    "--job-language": "python",
  },
});
