import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:lottie/lottie.dart';
import '../providers/auth_provider.dart';
import 'package:flutter_animate/flutter_animate.dart';

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

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnack(String msg) =>
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));

  InputDecoration _inputDec(String hint, IconData icon) => InputDecoration(
        filled: true,
        fillColor: Colors.white.withOpacity(0.9),
        hintText: hint,
        prefixIcon: Icon(icon, color: Colors.deepPurple),
        contentPadding: const EdgeInsets.symmetric(vertical: 14),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(color: Colors.deepPurple),
        ),
      );

  Widget _buildToggle() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: RegistrationMode.values.map((mode) {
        final selected = mode == _regMode;
        return AnimatedContainer(
          duration: 300.ms,
          margin: const EdgeInsets.symmetric(horizontal: 8),
          padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 8),
          decoration: BoxDecoration(
            color: selected ? Colors.deepPurple : Colors.white,
            borderRadius: BorderRadius.circular(20),
            boxShadow: selected
                ? [
                    BoxShadow(
                        color: Colors.deepPurple.withOpacity(0.4),
                        blurRadius: 8)
                  ]
                : null,
          ),
          child: InkWell(
            borderRadius: BorderRadius.circular(20),
            onTap: () => setState(() {
              _regMode = mode;
              _codeSent = false;
            }),
            child: Text(
              mode == RegistrationMode.email ? 'Email' : 'Phone',
              style: TextStyle(
                color: selected ? Colors.white : Colors.deepPurple,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        );
      }).toList(),
    );
  }

  Widget _buildText(String hint, IconData icon, TextEditingController ctrl,
      String? Function(String?) validator,
      {bool obscure = false, VoidCallback? toggle}) {
    return TextFormField(
      controller: ctrl,
      validator: validator,
      obscureText: obscure,
      decoration: _inputDec(hint, icon).copyWith(
        suffixIcon: toggle != null
            ? IconButton(
                icon: Icon(obscure ? Icons.visibility_off : Icons.visibility,
                    color: Colors.deepPurple),
                onPressed: toggle,
              )
            : null,
      ),
    );
  }

  Widget _buildRegister() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _regKey,
      child: ListView(
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
        children: [
          _buildToggle(),
          const SizedBox(height: 24),
          if (!_codeSent) ...[
            if (_regMode == RegistrationMode.email)
              _buildText(
                'Email',
                Icons.email,
                _emailCtrl,
                (v) => v!.isEmpty ? 'Введите email' : null,
              ),
            if (_regMode == RegistrationMode.phone)
              _buildText(
                'Phone',
                Icons.phone,
                _phoneCtrl,
                (v) => v!.isEmpty ? 'Введите телефон' : null,
              ),
            const SizedBox(height: 24),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.deepPurple,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
                padding: const EdgeInsets.symmetric(vertical: 16),
              ),
              onPressed: () async {
                if (_regKey.currentState!.validate()) {
                  setState(() => _loading = true);
                  try {
                    if (_regMode == RegistrationMode.email) {
                      await auth.sendEmailOtp(_emailCtrl.text.trim());
                      _showSnack('OTP отправлен на email');
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
              },
              child: const Text('Send OTP'),
            ),
          ],
          if (_codeSent) ...[
            _buildText(
              'Enter OTP',
              Icons.sms,
              _otpCtrl,
              (v) => v!.isEmpty ? 'Введите OTP' : null,
            ),
            const SizedBox(height: 16),
            _buildText(
              'Username',
              Icons.person,
              _usernameCtrl,
              (v) => v!.isEmpty ? 'Введите username' : null,
            ),
            const SizedBox(height: 16),
            _buildText(
              'Password',
              Icons.lock,
              _passCtrl,
              (v) => v!.length < 6 ? 'Минимум 6 символов' : null,
              obscure: !_passVisible,
              toggle: () => setState(() => _passVisible = !_passVisible),
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.deepPurple,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
                padding: const EdgeInsets.symmetric(vertical: 16),
              ),
              onPressed: () async {
                if (_regKey.currentState!.validate()) {
                  setState(() => _loading = true);
                  try {
                    if (_regMode == RegistrationMode.email) {
                      await auth.verifyEmailOtp(
                        _emailCtrl.text.trim(),
                        _otpCtrl.text.trim(),
                      );
                      await auth.registerWithEmail(
                        _usernameCtrl.text.trim(),
                        _emailCtrl.text.trim(),
                        _passCtrl.text.trim(),
                        _otpCtrl.text.trim(),
                      );
                    } else {
                      await auth.verifySmsOtp(
                        _phoneCtrl.text.trim(),
                        _otpCtrl.text.trim(),
                      );
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
              },
              child: const Text('Complete Registration'),
            ),
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
        padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 24),
        children: [
          _buildText(
            'Email/Username/Phone',
            Icons.person,
            _loginCtrl,
            (v) => v!.isEmpty ? 'Введите логин' : null,
          ),
          const SizedBox(height: 16),
          _buildText(
            'Password',
            Icons.lock,
            _loginPassCtrl,
            (v) => v!.length < 6 ? 'Минимум 6 символов' : null,
            obscure: !_passVisible,
            toggle: () => setState(() => _passVisible = !_passVisible),
          ),
          const SizedBox(height: 24),
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.deepPurple,
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12)),
              padding: const EdgeInsets.symmetric(vertical: 16),
            ),
            onPressed: () async {
              if (_loginKey.currentState!.validate()) {
                setState(() => _loading = true);
                try {
                  await auth.login(
                    _loginCtrl.text.trim(),
                    _loginPassCtrl.text.trim(),
                    context,
                  );
                } catch (e) {
                  _showSnack('Ошибка: $e');
                } finally {
                  setState(() => _loading = false);
                }
              }
            },
            child: const Text('Login'),
          ),
          const SizedBox(height: 12),
          TextButton(
            onPressed: () => Navigator.pushNamed(context, '/reset-password'),
            child: const Text('Забыли пароль?'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Stack(
        children: [
          Container(
            decoration: const BoxDecoration(
              gradient: LinearGradient(
                colors: [Color(0xFFEDE7F6), Color(0xFFD1C4E9)],
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
              ),
            ),
          ),
          Center(
            child: Lottie.asset(
              'assets/animation/logo_animation.json',
              width: 150,
              height: 150,
            ),
          ),
          Column(
            children: [
              const SizedBox(height: 280),
              Text('Добро пожаловать!',
                  style: const TextStyle(
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    color: Colors.deepPurple,
                  ))
                ..animate().fadeIn(delay: 500.ms),
              const SizedBox(height: 16),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 24),
                child: Card(
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(24),
                  ),
                  elevation: 12,
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      TabBar(
                        controller: _tabController,
                        indicator: BoxDecoration(
                          color: Colors.deepPurple,
                          borderRadius: BorderRadius.circular(24),
                        ),
                        labelColor: Colors.white,
                        unselectedLabelColor: Colors.deepPurple,
                        tabs: const [
                          Tab(text: 'Регистрация'),
                          Tab(text: 'Вход'),
                        ],
                      ),
                      SizedBox(
                        height: MediaQuery.of(context).size.height * 0.5,
                        child: TabBarView(
                          controller: _tabController,
                          children: [_buildRegister(), _buildLogin()],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          if (_loading)
            Container(
              color: Colors.black54,
              child: const Center(
                child: CircularProgressIndicator(
                  valueColor: AlwaysStoppedAnimation(Colors.white),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
