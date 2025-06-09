import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:lottie/lottie.dart';
import 'package:flutter_animate/flutter_animate.dart';
import '../providers/auth_provider.dart';

enum RegistrationMode { email, phone }

class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key});

  @override
  State<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  final _regKey = GlobalKey<FormState>();
  final _loginKey = GlobalKey<FormState>();
  final _usernameCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _otpCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  final _loginCtrl = TextEditingController();
  final _loginPassCtrl = TextEditingController();

  RegistrationMode _regMode = RegistrationMode.email;
  bool _codeSent = false;
  bool _loading = false;
  bool _passVisible = false;

  final Color primaryColor = const Color(0xFF30D5C8);

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  InputDecoration _inputDecoration(String hint, IconData icon) {
    return InputDecoration(
      filled: true,
      fillColor: Colors.white,
      hintText: hint,
      prefixIcon: Icon(icon, color: primaryColor),
      contentPadding: const EdgeInsets.symmetric(vertical: 16, horizontal: 20),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(16),
        borderSide: BorderSide.none,
      ),
    );
  }

  Widget _buildToggle() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: RegistrationMode.values.map((mode) {
        final selected = mode == _regMode;
        return GestureDetector(
          onTap: () => setState(() {
            _regMode = mode;
            _codeSent = false;
          }),
          child: AnimatedContainer(
            duration: 300.ms,
            margin: const EdgeInsets.symmetric(horizontal: 8),
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 10),
            decoration: BoxDecoration(
              color: selected ? primaryColor : Colors.white,
              borderRadius: BorderRadius.circular(30),
              border: Border.all(color: primaryColor, width: 1.5),
            ),
            child: Text(
              mode == RegistrationMode.email ? 'E-mail' : 'Телефон',
              style: TextStyle(
                color: selected ? Colors.white : primaryColor,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildTextField(String hint, IconData icon, TextEditingController ctrl,
      String? Function(String?) validator,
      {bool obscure = false, VoidCallback? toggle}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: TextFormField(
        controller: ctrl,
        validator: validator,
        obscureText: obscure,
        decoration: _inputDecoration(hint, icon).copyWith(
          suffixIcon: toggle != null
              ? IconButton(
                  icon: Icon(obscure ? Icons.visibility_off : Icons.visibility,
                      color: primaryColor),
                  onPressed: toggle,
                )
              : null,
        ),
      ),
    );
  }

  Widget _buildRegister() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _regKey,
      child: ListView(
        padding: const EdgeInsets.symmetric(horizontal: 24),
        children: [
          _buildToggle(),
          const SizedBox(height: 24),
          if (!_codeSent) ...[
            if (_regMode == RegistrationMode.email)
              _buildTextField('E-mail', Icons.email, _emailCtrl,
                  (v) => v!.isEmpty ? 'Введите e-mail' : null),
            if (_regMode == RegistrationMode.phone)
              _buildTextField('Телефон', Icons.phone, _phoneCtrl,
                  (v) => v!.isEmpty ? 'Введите номер' : null),
            const SizedBox(height: 24),
            _buildButton('Отправить код', () async {
              if (_regKey.currentState!.validate()) {
                setState(() => _loading = true);
                try {
                  if (_regMode == RegistrationMode.email) {
                    await auth.sendEmailOtp(_emailCtrl.text.trim());
                    _showSnack('OTP отправлен на e-mail');
                  } else {
                    await auth.sendSmsOtp(_phoneCtrl.text.trim());
                    _showSnack('OTP отправлен на телефон');
                  }
                  setState(() => _codeSent = true);
                } catch (e) {
                  _showSnack('Ошибка: $e');
                } finally {
                  setState(() => _loading = false);
                }
              }
            }),
          ],
          if (_codeSent) ...[
            _buildTextField('Код', Icons.sms, _otpCtrl,
                (v) => v!.isEmpty ? 'Введите код' : null),
            _buildTextField('Логин', Icons.person, _usernameCtrl,
                (v) => v!.isEmpty ? 'Введите логин' : null),
            _buildTextField(
              'Пароль',
              Icons.lock,
              _passCtrl,
              (v) => v!.length < 6 ? 'Минимум 6 символов' : null,
              obscure: !_passVisible,
              toggle: () => setState(() => _passVisible = !_passVisible),
            ),
            const SizedBox(height: 24),
            _buildButton('Завершить регистрацию', () async {
              if (_regKey.currentState!.validate()) {
                setState(() => _loading = true);
                try {
                  if (_regMode == RegistrationMode.email) {
                    await auth.verifyEmailOtp(
                        _emailCtrl.text.trim(), _otpCtrl.text.trim());
                    await auth.registerWithEmail(
                      _usernameCtrl.text.trim(),
                      _emailCtrl.text.trim(),
                      _passCtrl.text.trim(),
                      _otpCtrl.text.trim(),
                    );
                  } else {
                    await auth.verifySmsOtp(
                        _phoneCtrl.text.trim(), _otpCtrl.text.trim());
                    await auth.registerWithPhone(
                      _usernameCtrl.text.trim(),
                      _phoneCtrl.text.trim(),
                      _passCtrl.text.trim(),
                      _otpCtrl.text.trim(),
                    );
                  }
                  Navigator.pushReplacementNamed(context, '/main');
                } catch (e) {
                  _showSnack('Ошибка: $e');
                } finally {
                  setState(() => _loading = false);
                }
              }
            }),
          ],
        ],
      ),
    );
  }

  Widget _buildLogin() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _loginKey,
      child: ListView(
        padding: const EdgeInsets.symmetric(horizontal: 24),
        children: [
          _buildTextField('E-mail / Телефон / Логин', Icons.person, _loginCtrl,
              (v) => v!.isEmpty ? 'Введите логин' : null),
          _buildTextField(
            'Пароль',
            Icons.lock,
            _loginPassCtrl,
            (v) => v!.length < 6 ? 'Минимум 6 символов' : null,
            obscure: !_passVisible,
            toggle: () => setState(() => _passVisible = !_passVisible),
          ),
          const SizedBox(height: 24),
          _buildButton('Войти', () async {
            if (_loginKey.currentState!.validate()) {
              setState(() => _loading = true);
              try {
                await auth.login(_loginCtrl.text.trim(),
                    _loginPassCtrl.text.trim(), context);
              } catch (e) {
                _showSnack('Ошибка: $e');
              } finally {
                setState(() => _loading = false);
              }
            }
          }),
          TextButton(
            onPressed: () => Navigator.pushNamed(context, '/reset-password'),
            child: const Text('Забыли пароль?'),
          ),
        ],
      ),
    );
  }

  Widget _buildButton(String text, VoidCallback onPressed) {
    return ElevatedButton(
      style: ElevatedButton.styleFrom(
        backgroundColor: primaryColor,
        padding: const EdgeInsets.symmetric(vertical: 16),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        elevation: 4,
      ),
      onPressed: onPressed,
      child: Text(text, style: const TextStyle(fontSize: 16)),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFFF3F7FA),
      body: Stack(
        children: [
          if (_loading)
            Container(
              color: Colors.black45,
              child: const Center(
                child: CircularProgressIndicator(color: Colors.white),
              ),
            ),
          Column(
            children: [
              const SizedBox(height: 100),
              Lottie.asset('assets/animation/loading_animation.json',
                  width: 160, height: 160),
              const SizedBox(height: 8),
              Text(
                'Добро пожаловать!',
                style: TextStyle(
                  fontSize: 26,
                  fontWeight: FontWeight.bold,
                  color: primaryColor,
                ),
              ).animate().fadeIn(duration: 600.ms),
              const SizedBox(height: 24),
              Expanded(
                child: Container(
                  margin: const EdgeInsets.symmetric(horizontal: 20),
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(32),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black12,
                        blurRadius: 12,
                        offset: const Offset(0, 8),
                      ),
                    ],
                  ),
                  child: Column(
                    children: [
                      const SizedBox(height: 16),
                      TabBar(
                        controller: _tabController,
                        indicator: BoxDecoration(
                          color: primaryColor,
                          borderRadius: BorderRadius.circular(24),
                        ),
                        labelColor: Colors.white,
                        unselectedLabelColor: primaryColor,
                        tabs: const [
                          Tab(text: 'Регистрация'),
                          Tab(text: 'Вход'),
                        ],
                      ),
                      Expanded(
                        child: TabBarView(
                          controller: _tabController,
                          children: [_buildRegister(), _buildLogin()],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 20),
            ],
          ),
        ],
      ),
    );
  }
}
