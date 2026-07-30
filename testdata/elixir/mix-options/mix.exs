defmodule Options.MixProject do
  use Mix.Project

  def project, do: [app: :options, version: "0.1.0", deps: deps()]

  defp deps do
    [
      {:ex_doc, "~> 0.34", only: :dev, runtime: false},
      {:credo, "~> 1.7", only: [:dev, :test], optional: true},
      {:telemetry, "~> 1.2", override: true, targets: [:host, :rpi3]}
    ]
  end
end
