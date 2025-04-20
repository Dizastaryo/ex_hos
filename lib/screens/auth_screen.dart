import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:lottie/lottie.dart';
import '../providers/auth_provider.dart';
import 'package:flutter_animate/flutter_animate.dart';

enum RegistrationMode { email, phone }

class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key});

  @override
  _AuthScreenState createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  final _regFormKey = GlobalKey<FormState>();
  final _regUsernameController = TextEditingController();
  final _regEmailController = TextEditingController();
  final _regPhoneController = TextEditingController();
  final _regOtpController = TextEditingController();
  final _regPasswordController = TextEditingController();
  RegistrationMode _regMode = RegistrationMode.email;
  bool _isRegCodeSent = false;

  final _loginFormKey = GlobalKey<FormState>();
  final _loginController = TextEditingController();
  final _loginPasswordController = TextEditingController();
  bool _isPasswordVisible = false;

  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(msg),
        backgroundColor: Theme.of(context).primaryColor,
      ),
    );
  }

  Widget _buildTextField({
    required String hint,
    required IconData icon,
    required TextEditingController controller,
    required String? Function(String?) validator,
    bool obscure = false,
    VoidCallback? toggleVisibility,
  }) {
    return TextFormField(
      controller: controller,
      obscureText: obscure,
      validator: validator,
      decoration: InputDecoration(
        hintText: hint,
        prefixIcon: Icon(icon, color: Theme.of(context).primaryColor),
        suffixIcon: toggleVisibility == null
            ? null
            : IconButton(
                icon: Icon(
                  obscure ? Icons.visibility_off : Icons.visibility,
                  color: Theme.of(context).primaryColor,
                ),
                onPressed: toggleVisibility,
              ),
        filled: true,
        fillColor: Colors.grey[100],
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
      ),
    );
  }

  Widget _buildRegisterTab() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _regFormKey,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                ChoiceChip(
                  label: const Text('Email'),
                  selected: _regMode == RegistrationMode.email,
                  onSelected: (_) => setState(() {
                    _regMode = RegistrationMode.email;
                    _isRegCodeSent = false;
                  }),
                ),
                const SizedBox(width: 12),
                ChoiceChip(
                  label: const Text('Phone'),
                  selected: _regMode == RegistrationMode.phone,
                  onSelected: (_) => setState(() {
                    _regMode = RegistrationMode.phone;
                    _isRegCodeSent = false;
                  }),
                ),
              ],
            ),
            const SizedBox(height: 24),
            if (!_isRegCodeSent) ...[
              if (_regMode == RegistrationMode.email)
                _buildTextField(
                  hint: 'Email',
                  icon: Icons.email,
                  controller: _regEmailController,
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Введите email';
                    if (!RegExp(r'^[^@]+@[^@]+\.[^@]+').hasMatch(v))
                      return 'Некорректный email';
                    return null;
                  },
                ),
              if (_regMode == RegistrationMode.phone)
                _buildTextField(
                  hint: 'Phone (+7...)',
                  icon: Icons.phone,
                  controller: _regPhoneController,
                  validator: (v) =>
                      v == null || v.isEmpty ? 'Введите телефон' : null,
                ),
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: () async {
                  if (_regFormKey.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      if (_regMode == RegistrationMode.email) {
                        await auth
                            .sendEmailOtp(_regEmailController.text.trim());
                        _showSnack('OTP sent to email');
                      } else {
                        await auth.sendSmsOtp(_regPhoneController.text.trim());
                        _showSnack('OTP sent to phone');
                      }
                      setState(() => _isRegCodeSent = true);
                    } catch (e) {
                      _showSnack('Ошибка: \$e');
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(50),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                ),
                child: Text('Send OTP'.toUpperCase()),
              ),
            ],
            if (_isRegCodeSent) ...[
              _buildTextField(
                hint: 'Username',
                icon: Icons.person,
                controller: _regUsernameController,
                validator: (v) =>
                    v == null || v.isEmpty ? 'Введите имя пользователя' : null,
              ),
              const SizedBox(height: 16),
              _buildTextField(
                hint: 'Enter OTP',
                icon: Icons.sms,
                controller: _regOtpController,
                validator: (v) => v == null || v.isEmpty ? 'Введите OTP' : null,
              ),
              const SizedBox(height: 16),
              _buildTextField(
                hint: 'Password',
                icon: Icons.lock,
                controller: _regPasswordController,
                obscure: !_isPasswordVisible,
                toggleVisibility: () =>
                    setState(() => _isPasswordVisible = !_isPasswordVisible),
                validator: (v) {
                  if (v == null || v.isEmpty) return 'Введите пароль';
                  if (v.length < 6) return 'Минимум 6 символов';
                  return null;
                },
              ),
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: () async {
                  if (_regFormKey.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      if (_regMode == RegistrationMode.email) {
                        await auth.verifyEmailOtp(
                          _regEmailController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                        await auth.registerWithEmail(
                          _regUsernameController.text.trim(),
                          _regEmailController.text.trim(),
                          _regPasswordController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                      } else {
                        await auth.verifySmsOtp(
                          _regPhoneController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                        await auth.registerWithPhone(
                          _regUsernameController.text.trim(),
                          _regPhoneController.text.trim(),
                          _regPasswordController.text.trim(),
                          _regOtpController.text.trim(),
                        );
                      }
                      Navigator.pushReplacementNamed(context, '/main');
                    } catch (e) {
                      _showSnack('Ошибка: \$e');
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
                style: ElevatedButton.styleFrom(
                  minimumSize: const Size.fromHeight(50),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                ),
                child: Text('Complete Registration'.toUpperCase()),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildLoginTab() {
    final auth = Provider.of<AuthProvider>(context, listen: false);
    return Form(
      key: _loginFormKey,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            _buildTextField(
              hint: 'Email/Username/Phone',
              icon: Icons.person,
              controller: _loginController,
              validator: (v) => v == null || v.isEmpty ? 'Введите логин' : null,
            ),
            const SizedBox(height: 16),
            _buildTextField(
              hint: 'Password',
              icon: Icons.lock,
              controller: _loginPasswordController,
              obscure: !_isPasswordVisible,
              toggleVisibility: () =>
                  setState(() => _isPasswordVisible = !_isPasswordVisible),
              validator: (v) {
                if (v == null || v.isEmpty) return 'Введите пароль';
                if (v.length < 6) return 'Минимум 6 символов';
                return null;
              },
            ),
            const SizedBox(height: 20),
            ElevatedButton(
              onPressed: () async {
                if (_loginFormKey.currentState?.validate() ?? false) {
                  setState(() => _isLoading = true);
                  try {
                    await auth.login(
                      _loginController.text.trim(),
                      _loginPasswordController.text.trim(),
                      context,
                    );
                  } catch (e) {
                    _showSnack('Ошибка: \$e');
                  } finally {
                    setState(() => _isLoading = false);
                  }
                }
              },
              style: ElevatedButton.styleFrom(
                minimumSize: const Size.fromHeight(50),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12)),
              ),
              child: Text('Login'.toUpperCase()),
            ),
            const SizedBox(height: 12),
            TextButton(
              onPressed: () => Navigator.pushNamed(context, '/my-resetpas'),
              child: Text(
                'Забыли пароль?',
                style: TextStyle(color: Theme.of(context).primaryColor),
              ),
            ),
          ],
        ),
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
                colors: [Color(0xFF6A11CB), Color(0xFF2575FC)],
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
              ),
            ),
            child: SafeArea(
              child: Center(
                child: SingleChildScrollView(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 24, vertical: 32),
                  child: Container(
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      color: Colors.white,
                      borderRadius: BorderRadius.circular(20),
                      boxShadow: [
                        BoxShadow(
                            color: Colors.black26,
                            blurRadius: 12,
                            offset: Offset(0, 6))
                      ],
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Animate(
                          effects: [FadeEffect(duration: 600.ms)],
                          child: Text(
                            'Добро пожаловать!',
                            style: TextStyle(
                              fontSize: 24,
                              fontWeight: FontWeight.bold,
                              color: Theme.of(context).primaryColor,
                            ),
                          ),
                        ),
                        const SizedBox(height: 12),
                        TabBar(
                          controller: _tabController,
                          indicatorColor: Theme.of(context).primaryColor,
                          labelColor: Theme.of(context).primaryColor,
                          unselectedLabelColor:
                              Theme.of(context).primaryColor.withOpacity(0.6),
                          tabs: const [
                            Tab(text: 'Регистрация'),
                            Tab(text: 'Вход')
                          ],
                        ),
                        const SizedBox(height: 16),
                        SizedBox(
                          height: 400,
                          child: TabBarView(
                            controller: _tabController,
                            children: [_buildRegisterTab(), _buildLoginTab()],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
          if (_isLoading)
            Center(
              child: Lottie.asset(
                'assets/animation/loading_animation.json',
                width: 100,
                height: 100,
              ),
            ),
        ],
      ),
    );
  }
}
