import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import '../model/user_dto.dart'; // Импортируем DTO

class UserService {
  // Берём базовый URL аутентификации из .env и дописываем путь к users
  final String _baseUrl = '${dotenv.env['AUTH_BASE_URL']!}/api/test/users';
  final Dio _dio;

  UserService(this._dio);

  Future<String> getUsernameById(int userId) async {
    try {
      final response = await _dio.get('$_baseUrl/$userId/username');
      return response.data as String;
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  Future<List<UserDTO>> getAllDoctors() async {
    try {
      final response = await _dio.get('$_baseUrl/moderators');
      return (response.data as List)
          .map((json) => UserDTO.fromJson(json))
          .toList();
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  /// Создать модератора
  Future<String> createModerator({
    required String username,
    required String email,
    required String password,
  }) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/create-moderator',
        data: {
          "username": username,
          "email": email,
          "password": password,
        },
      );
      return response.data as String;
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  /// Поиск пользователей
  Future<List<UserDTO>> searchUsers(String query) async {
    try {
      final response = await _dio.get(
        '$_baseUrl/search',
        queryParameters: {'query': query},
      );
      return (response.data as List)
          .map((json) => UserDTO.fromJson(json))
          .toList();
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  /// Блокировка пользователя
  Future<String> blockUser(int userId) async {
    try {
      final response = await _dio.put('$_baseUrl/block/$userId');
      return response.data as String;
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  /// Разблокировка пользователя
  Future<String> unblockUser(int userId) async {
    try {
      final response = await _dio.put('$_baseUrl/unblock/$userId');
      return response.data as String;
    } on DioException catch (e) {
      throw Exception(_formatError(e));
    }
  }

  String _formatError(DioException e) {
    if (e.response != null) {
      final statusCode = e.response?.statusCode;
      final data = e.response?.data;
      if (statusCode == 401) {
        return 'Ошибка авторизации (401). Пожалуйста, перезайдите.';
      }
      return 'Ошибка $statusCode: $data';
    } else {
      return 'Сетевая ошибка: ${e.message}';
    }
  }
}
