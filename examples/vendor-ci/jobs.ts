import { CfnJob } from "aws-cdk-lib/aws-glue";

new CfnJob(this, "typescript-current", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "idna==3.10,urllib3==2.2.3",
  },
});

new CfnJob(this, "typescript-legacy", {
  defaultArguments: {
    "--job-language": "python",
    "--additional-python-modules": "idna==3.10,urllib3==1.26.20",
  },
});
