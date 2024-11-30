import 'package:flutter/material.dart';
import '../services/auth_service.dart';
import 'package:lottie/lottie.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';

class AuthProvider with ChangeNotifier {
  final AuthService _authService;
  bool _isCodeSent = false;
  bool _isLoading = false;
  dynamic currentUser;

  AuthProvider(Dio dio, CookieJar cookieJar) : _authService = AuthService(dio);

  bool get isCodeSent => _isCodeSent;
  bool get isLoading => _isLoading;

  // Function to send OTP
  Future<void> signInWithEmail(String email) async {
    try {
      _setLoading(true);
      await _authService.sendOtp(email);
      _isCodeSent = true;
      notifyListeners();
    } catch (e) {
      throw Exception('Error sending OTP: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Function to verify OTP
  Future<void> signInWithOtp(String email, String otp) async {
    try {
      _setLoading(true);
      await _authService.verifyOtp(email, otp);
      currentUser = {'email': email};
      _isCodeSent = false;
      notifyListeners();
    } catch (e) {
      throw Exception('Error verifying OTP: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Function for registration
  Future<void> completeRegistration(
      String email, String password, String role, String status) async {
    try {
      _setLoading(true);
      await _authService.registerUser(email, password, role, status);
      currentUser = {'email': email, 'role': role, 'status': status};
      notifyListeners();
    } catch (e) {
      throw Exception('Error completing registration: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Function for login
  Future<void> login(
      BuildContext context, String email, String password) async {
    try {
      _setLoading(true);
      await _authService.login(email, password);
      currentUser = {'email': email};
      notifyListeners();
      Navigator.pushReplacementNamed(context, '/main');
    } catch (e) {
      throw Exception('Login error: $e');
    } finally {
      _setLoading(false);
    }
  }

  // Function for sign-out
  Future<void> signOut() async {
    _isCodeSent = false;
    currentUser = null;
    notifyListeners();
  }

  void _setLoading(bool isLoading) {
    _isLoading = isLoading;
    notifyListeners();
  }

  Widget _buildLoadingIndicator() {
    return Lottie.asset(
      'assets/loading_animation.json', // path to animation file
      width: 100,
      height: 100,
      fit: BoxFit.fill,
    );
  }
}
