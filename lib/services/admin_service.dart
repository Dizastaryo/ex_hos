import 'package:dio/dio.dart';

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

class UserService {
  final Dio _dio;

  UserService(this._dio);
  
  Future<String> createModerator({
    required String username,
    required String email,
    required String password,
  }) async {
    final response = await _dio.post(
      'http://172.20.10.2:8081/api/test/users/create-moderator',
      data: {
        "username": username,
        "email": email,
        "password": password,
      },
    );
    return response
        .data; // "Moderator created successfully!" или сообщение об ошибке
  }

  Future<List<UserDTO>> searchUsers(String query) async {
    final response = await _dio.get(
      'http://172.20.10.2:8081/api/test/users/search',
      queryParameters: {
        'query': query,
      },
    );
    return (response.data as List)
        .map((json) => UserDTO.fromJson(json))
        .toList();
  }

  // Метод для блокировки пользователя
  Future<String> blockUser(int userId) async {
    try {
      final response = await _dio.put(
        'http://172.20.10.2:8081/api/test/users/block/$userId',
      );
      return response
          .data; // "User blocked successfully!" или сообщение об ошибке
    } catch (e) {
      return "Error blocking user: $e";
    }
  }

  // Метод для разблокировки пользователя
  Future<String> unblockUser(int userId) async {
    try {
      final response = await _dio.put(
        'http://172.20.10.2:8081/api/test/users/unblock/$userId',
      );
      return response
          .data; // "User unblocked successfully!" или сообщение об ошибке
    } catch (e) {
      return "Error unblocking user: $e";
    }
  }
}
