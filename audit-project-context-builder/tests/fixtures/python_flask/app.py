from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route("/users", methods=["GET"])
def list_users():
    return jsonify({"users": []})

@app.route("/users", methods=["POST"])
def create_user():
    data = request.get_json()
    return jsonify({"id": "new", "name": data["name"]}), 201

@app.route("/users/<int:user_id>", methods=["GET"])
def get_user(user_id):
    return jsonify({"id": user_id})

@app.route('/users/<int:user_id>', methods=['DELETE'])
def delete_user(user_id):
    return '', 204

if __name__ == "__main__":
    app.run()
