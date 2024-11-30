// lib/services/auth_service.dart
import 'package:dio/dio.dart';

class AuthService {
  final Dio dio;

  AuthService(this.dio);

  // Функция отправки OTP
  Future<void> sendOtp(String email) async {
    try {
      final response = await dio.post(
        'http://172.20.10.3:8080/user-api/v1/auth/sendCode',
        data: {'email': email},
      );

      if (response.statusCode == 200) {
        print('OTP sent to $email');
      } else {
        throw Exception('Error sending OTP: ${response.data}');
      }
    } catch (e) {
      throw Exception('Error sending OTP: $e');
    }
  }

  // Функция логина
  Future<void> login(String email, String password) async {
    try {
      final response = await dio.post(
        'http://172.20.10.3:8080/user-api/v1/auth/login',
        data: {'email': email, 'password': password},
      );

      if (response.statusCode == 200) {
        print(response.headers); // Проверьте, содержит ли "Set-Cookie".
        print('Login successful for $email');
      } else {
        throw Exception('Login error: ${response.data}');
      }
    } catch (e) {
      throw Exception('Login error: $e');
    }
  }

  // Функция проверки OTP
  Future<void> verifyOtp(String email, String otp) async {
    try {
      final response = await dio.post(
        'http://172.20.10.3:8080/user-api/v1/auth/validateCode',
        data: {'email': email, 'code': otp},
      );

      if (response.statusCode == 200) {
        print('OTP verified');
      } else {
        throw Exception('Error verifying OTP: ${response.data}');
      }
    } catch (e) {
      throw Exception('Error verifying OTP: $e');
    }
  }

  // Функция регистрации пользователя
  Future<void> registerUser(
      String email, String password, String role, String status) async {
    try {
      final response = await dio.post(
        'http://172.20.10.3:8080/user-api/v1/auth/registration',
        data: {
          'email': email,
          'password': password,
          'role': role,
          'status': status,
        },
      );

      if (response.statusCode == 200 || response.statusCode == 500) {
        print('User registered successfully');
      } else {
        throw Exception('Registration error: ${response.data}');
      }
    } catch (e) {
      throw Exception('Error during registration: $e');
    }
  }
}
