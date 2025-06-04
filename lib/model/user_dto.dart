// lib/models/user_dto.dart

class UserDTO {
  final int id;
  final String username;

  UserDTO({required this.id, required this.username});

  factory UserDTO.fromJson(Map<String, dynamic> json) {
    return UserDTO(
      id: json['id'],
      username: json['username'],
    );
  }
}
