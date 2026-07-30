defmodule Git.MixProject do
  use Mix.Project

  def project, do: [app: :git_sources, version: "0.1.0", deps: deps()]

  defp deps do
    [
      {:gettext, git: "https://github.com/elixir-gettext/gettext.git", tag: "v0.26.2"},
      {:nimble_parsec, github: "dashbitco/nimble_parsec", branch: "main", submodules: true},
      {:phoenix_html, git: "https://github.com/phoenixframework/phoenix_html.git", ref: "aabbccddeeff00112233445566778899aabbccdd", depth: 1, subdir: "phoenix_html"}
    ]
  end
end
