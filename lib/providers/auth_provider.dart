import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import '../services/auth_service.dart';
import 'package:encrypt/encrypt.dart' as encrypt;

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

  // Ключ для шифрования (32 байта = 256 бит)
  final _encryptionKey = encrypt.Key.fromUtf8(
      '32charslongencryptionkey32charslongencryptionkey'); // 32 байта = 256 бит
  final _iv =
      encrypt.IV.fromLength(16); // Инициализационный вектор длиной 16 байт

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  // Шифрование текста
  String _encryptText(String text) {
    final encrypter = encrypt.Encrypter(encrypt.AES(_encryptionKey));
    return encrypter.encrypt(text, iv: _iv).base64;
  }

  // Дешифрование текста
  String _decryptText(String encryptedText) {
    final encrypter = encrypt.Encrypter(encrypt.AES(_encryptionKey));
    return encrypter.decrypt64(encryptedText, iv: _iv);
  }

  Future<void> _saveCredentials(String login, String password) async {
    final prefs = await SharedPreferences.getInstance();
    String encryptedLogin = _encryptText(login);
    String encryptedPassword = _encryptText(password);
    await prefs.setString('login', encryptedLogin);
    await prefs.setString('password', encryptedPassword);
  }

  Future<Map<String, String>?> _loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    final encryptedLogin = prefs.getString('login');
    final encryptedPassword = prefs.getString('password');

    if (encryptedLogin != null && encryptedPassword != null) {
      String login = _decryptText(encryptedLogin);
      String password = _decryptText(encryptedPassword);
      return {'login': login, 'password': password};
    }
    return null;
  }

  Future<void> _clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
    await _cookieJar.deleteAll();
  }

  String? getDecryptedLogin() {
    if (currentUser != null) {
      final encryptedLogin = currentUser['login'];
      return encryptedLogin != null ? _decryptText(encryptedLogin) : null;
    }
    return null;
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
      await _clearCredentials();
      _token = null;
      currentUser = null;
      notifyListeners();
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
