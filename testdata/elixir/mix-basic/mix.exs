defmodule Basic.MixProject do
  use Mix.Project

  def project, do: [app: :basic, version: "0.1.0", deps: deps()]

  defp deps do
    [{:plug_cowboy, "~> 2.7"}, {:jason, "~> 1.4"}]
  end
end
