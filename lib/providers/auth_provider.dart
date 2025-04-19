import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import '../services/auth_service.dart';

class AuthProvider with ChangeNotifier {
  final AuthService _authService;
  bool _isLoading = false;
  dynamic currentUser;
  String? _token;

  AuthProvider(Dio dio) : _authService = AuthService(dio);

  bool get isLoading => _isLoading;
  String? get token => _token;

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  Future<void> _saveCredentials(
      String login, String password, String token) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('login', login);
    await prefs.setString('password', password);
    await prefs.setString('token', token);
  }

  Future<Map<String, String>?> _loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    final login = prefs.getString('login');
    final password = prefs.getString('password');
    final token = prefs.getString('token');

    if (login != null && password != null && token != null) {
      return {'login': login, 'password': password, 'token': token};
    }
    return null;
  }

  Future<void> _clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
  }

  // Вход
  Future<void> login(String login, String password,
      [BuildContext? context]) async {
    try {
      _setLoading(true);
      final response = await _authService.login(login, password);

      if (response.statusCode == 200) {
        final data = response.data;
        _token = data['accessToken'];
        currentUser = data;
        await _saveCredentials(login, password, _token!);
        notifyListeners();

        if (context != null) {
          Navigator.pushReplacementNamed(context, '/main');
        }
      }
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Автоматический вход
  Future<void> autoLogin(BuildContext context) async {
    final credentials = await _loadCredentials();
    if (credentials != null) {
      await login(credentials['login']!, credentials['password']!, context);
    } else {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        Navigator.pushReplacementNamed(context, '/auth');
      });
    }
  }

  // Выход
  Future<void> logout() async {
    try {
      await _authService.logout();
    } catch (_) {}
    await _clearCredentials();
    _token = null;
    currentUser = null;
    notifyListeners();
  }

  // Обновление токена
  Future<void> refreshToken() async {
    try {
      final response = await _authService.refreshToken();
      if (response.statusCode == 200) {
        _token = response.data['accessToken'];
        notifyListeners();
      }
    } catch (e) {
      throw Exception('Token refresh error: $e');
    }
  }

  // Восстановление пароля
  Future<void> requestPasswordReset(String login) async {
    await _authService.requestPasswordReset(login);
  }

  Future<void> confirmPasswordReset(
      String login, String otp, String newPassword) async {
    await _authService.confirmPasswordReset(login, otp, newPassword);
  }

  // Прокси к AuthService
  Future<void> sendEmailOtp(String email) => _authService.sendEmailOtp(email);
  Future<void> verifyEmailOtp(String email, String otp) =>
      _authService.verifyEmailOtp(email, otp);
  Future<void> sendSmsOtp(String phone) => _authService.sendSmsOtp(phone);
  Future<void> verifySmsOtp(String phone, String otp) =>
      _authService.verifySmsOtp(phone, otp);
  Future<void> registerWithEmail(
          String username, String email, String password, String otp) =>
      _authService.registerWithEmail(username, email, password, otp);
  Future<void> registerWithPhone(
          String username, String phone, String password, String otp) =>
      _authService.registerWithPhone(username, phone, password, otp);
}
