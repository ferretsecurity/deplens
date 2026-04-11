defmodule Demo.MixProject do
  use Mix.Project

  def project do
    [
      app: :demo,
      version: "0.1.0",
      deps: deps()
    ]
  end

  defp deps do
    [
      {:plug_cowboy, "~> 2.7"}
    ]
  end
end
