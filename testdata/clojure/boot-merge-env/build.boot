(require '[boot.core :as boot])

(boot/merge-env!
 :dependencies '[[io.aviso/pretty "1.1.1"]
                 [org.clojure/data.json "2.5.1"]])
