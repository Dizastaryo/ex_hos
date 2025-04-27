class Profile {
  final int id;
  final String username;
  final String role;

  Profile({
    required this.id,
    required this.username,
    required this.role,
  });

  factory Profile.fromJson(Map<String, dynamic> json) {
    return Profile(
      id: json['id'],
      username: json['username'],
      role: json['role'],
    );
  }
}
