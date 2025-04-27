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

  // ВАЖНО: ровно 32 символа = 256 бит
  final _encryptionKey =
      encrypt.Key.fromUtf8('my32charlongsecretkey1234567890!' // 32 chars
          );
  final _iv = encrypt.IV.fromLength(16); // 16 байт = 128 бит

  void _setLoading(bool value) {
    _isLoading = value;
    notifyListeners();
  }

  // Шифрование
  String _encryptText(String text) {
    final encrypter = encrypt.Encrypter(encrypt.AES(_encryptionKey));
    return encrypter.encrypt(text, iv: _iv).base64;
  }

  // Дешифрование
  String _decryptText(String encryptedText) {
    final encrypter = encrypt.Encrypter(encrypt.AES(_encryptionKey));
    return encrypter.decrypt64(encryptedText, iv: _iv);
  }

  Future<void> _saveCredentials(String login, String password) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('login', _encryptText(login));
    await prefs.setString('password', _encryptText(password));
  }

  Future<Map<String, String>?> _loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    final eLogin = prefs.getString('login');
    final ePass = prefs.getString('password');
    if (eLogin != null && ePass != null) {
      return {
        'login': _decryptText(eLogin),
        'password': _decryptText(ePass),
      };
    }
    return null;
  }

  Future<void> _clearCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.clear();
    await _cookieJar.deleteAll();
  }

  String? getDecryptedLogin() {
    if (currentUser != null && currentUser['login'] != null) {
      return _decryptText(currentUser['login']);
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
        if (context != null) {
          final roles = List<String>.from(response.data['roles']);
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
    final creds = await _loadCredentials();
    if (creds != null) {
      await login(creds['login']!, creds['password']!, context);
    } else {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        Navigator.pushReplacementNamed(context, '/auth');
      });
    }
  }

  Future<void> logout(BuildContext context) async {
    await _clearCredentials();
    _token = null;
    currentUser = null;
    notifyListeners();
    Navigator.pushReplacementNamed(context, '/auth');
  }

  Future<void> refreshToken() async {
    final response = await _dio.post('/auth/refresh');
    if (response.statusCode == 200) {
      _token = response.data['accessToken'];
      notifyListeners();
    }
  }

  Future<void> requestPasswordReset(String login) =>
      _authService.requestPasswordReset(login);

  Future<void> confirmPasswordReset(
          String login, String otp, String newPassword) =>
      _authService.confirmPasswordReset(login, otp, newPassword);

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
