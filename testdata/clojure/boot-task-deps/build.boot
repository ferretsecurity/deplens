(require '[boot.core :refer :all])

(deftask development []
  (set-env! :dependencies #(conj % '[reloaded.repl "0.2.4"]))
  (set-env! :dependencies #(conj % '[org.clojure/tools.namespace "1.5.0"]))
  identity)
