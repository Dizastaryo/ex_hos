import 'package:dio/dio.dart';

class AuthService {
  final Dio dio;

  AuthService(this.dio);

  // Функция отправки OTP для регистрации
  Future<void> sendOtp(String email) async {
    try {
      final response = await dio.post(
        'http://172.20.10.6:8080/user-api/v1/auth/sendCode',
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

  // Функция для проверки OTP
  Future<void> verifyOtp(String email, String otp) async {
    try {
      final response = await dio.post(
        'http://172.20.10.6:8080/user-api/v1/auth/validateCode',
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

  // Функция для регистрации с новым паролем и номером телефона
  Future<void> completeRegistration(
      String email, String password, String phoneNumber) async {
    try {
      final response = await dio.post(
        'http://172.20.10.6:8080/user-api/v1/auth/registration',
        data: {
          'email': email,
          'password': password,
          'phoneNumber': phoneNumber,
          'role': 'USER', // Статичный роль
          'status': 'OPEN' // Статичный статус
        },
      );

      if (response.statusCode == 200) {
        print('User registered successfully');
      } else {
        throw Exception('Registration error: ${response.data}');
      }
    } catch (e) {
      throw Exception('Error completing registration: $e');
    }
  }

  // Функция логина с email и паролем
  Future<void> login(String email, String password) async {
    try {
      final response = await dio.post(
        'http://172.20.10.6:8080/user-api/v1/auth/login',
        data: {'email': email, 'password': password},
      );

      if (response.statusCode == 200) {
        print('Login successful');
      } else {
        throw Exception('Login error: ${response.data}');
      }
    } catch (e) {
      throw Exception('Login error: $e');
    }
  }
}
