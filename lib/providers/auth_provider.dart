import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import '../services/auth_service.dart';

class AuthProvider with ChangeNotifier {
  final Dio _dio;
  final CookieJar _cookieJar;
  final AuthService _authService;
  bool _isLoading = false;
  dynamic currentUser;
  String? _token;

  AuthProvider(this._dio, this._cookieJar) : _authService = AuthService(_dio);

  bool get isLoading => _isLoading;
  String? get token => _token;

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  Future<void> _saveCredentials(String login, String password) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('login', login);
    await prefs.setString('password', password);
  }

  Future<Map<String, String>?> _loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    final login = prefs.getString('login');
    final password = prefs.getString('password');

    if (login != null && password != null) {
      return {'login': login, 'password': password};
    }
    return null;
  }

  Future<void> _clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
    await _cookieJar.deleteAll();
  }

  Future<void> login(String login, String password,
      [BuildContext? context]) async {
    try {
      _setLoading(true);
      final response = await _authService.login(login, password);

      if (response.statusCode == 200) {
        _token = response.data['accessToken'];
        currentUser = response.data;
        await _saveCredentials(login, password);
        notifyListeners();

        final roles = List<String>.from(response.data['roles']);
        if (context != null) {
          _navigateBasedOnRole(context, roles);
        }
      }
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  void _navigateBasedOnRole(BuildContext context, List<String> roles) {
    final route = roles.contains('ROLE_ADMIN')
        ? '/admin-home'
        : roles.contains('ROLE_MODERATOR')
            ? '/moderator-home'
            : '/main';

    Navigator.pushReplacementNamed(context, route);
  }

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

  Future<void> logout(BuildContext context) async {
    try {
      // Просто очищаем данные и переходим на экран авторизации
      await _clearCredentials();
      _token = null;
      currentUser = null;
      notifyListeners();

      // Переход в экран авторизации
      Navigator.pushReplacementNamed(context, '/auth');
    } catch (e) {
      throw Exception('Logout error: $e');
    }
  }

  Future<void> refreshToken() async {
    try {
      final response = await _dio.post('/auth/refresh');
      if (response.statusCode == 200) {
        _token = response.data['accessToken'];
        notifyListeners();
      }
    } catch (e) {
      throw Exception('Token refresh failed: $e');
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
