defmodule Regex.MixProject do
  use Mix.Project

  def project, do: [app: :regex_requirements, version: "0.1.0", deps: deps()]

  defp deps do
    [
      {:legacy, ~r/^1\.(2|3)\./, compile: "make", system_env: [{"FEATURE", "enabled"}]},
      {:private_client, "~> 1.0", warn_if_outdated: false, targets: :host}
    ]
  end
end
