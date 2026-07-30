defmodule Sparse.MixProject do
  use Mix.Project

  def project, do: [app: :sparse_git, version: "0.1.0", deps: deps()]

  defp deps do
    [
      {:component, git: "https://github.com/example/monorepo.git", tag: "v2.0.0", sparse: "apps/component", submodules: true}
    ]
  end
end
