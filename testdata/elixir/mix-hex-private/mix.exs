defmodule Private.MixProject do
  use Mix.Project

  def project, do: [app: :private_hex, version: "0.1.0", deps: deps()]

  defp deps do
    [
      {:my_client, "~> 2.0", hex: :company_client, repo: "company"},
      {:ssl_verify_fun, ">= 1.1.6 and < 2.0.0", manager: :rebar3, app: false}
    ]
  end
end
