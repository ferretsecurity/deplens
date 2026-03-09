import { CfnJob } from "aws-cdk-lib/aws-glue";

const props = {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "pandas==2.2.1",
  },
};

new CfnJob(this, "Job", props);
