defmodule Example.MixProject do
  use Mix.Project

  defp deps do
    [
      {:benchee, "~> 1.2", only: :bench},
      {:credo, "~> 1.7", only: [:dev, :test]},
      {:dialyxir, "~> 1.4", only: [:dev], runtime: false},
      {:ex_doc, "~> 0.34", only: :dev},
      {:hammer, "~> 7.0"},
      {:redix, "~> 1.5"}
    ]
  end
end
