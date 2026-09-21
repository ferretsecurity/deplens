resource "aws_glue_job" "daily" {
  default_arguments = {
    "--additional-python-modules" = "pandas==2.2.1,paramiko"
  }
}

resource "aws_glue_job" "legacy" {
  default_arguments = {
    "--job-language"              = "python"
    "--additional-python-modules" = "pandas==0.25.3"
  }
}
