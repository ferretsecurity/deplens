defmodule Local.MixProject do
  use Mix.Project

  def project, do: [app: :local_sources, version: "0.1.0", apps_path: "apps", deps: deps()]

  defp deps do
    [
      {:shared, path: "../shared", env: :dev},
      {:accounts, in_umbrella: true}
    ]
  end
end
