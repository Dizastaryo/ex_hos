import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:lottie/lottie.dart'; // Importing Lottie
import '../providers/auth_provider.dart';
import 'package:flutter_animate/flutter_animate.dart';

class AuthScreen extends StatefulWidget {
  const AuthScreen({super.key});

  @override
  _AuthScreenState createState() => _AuthScreenState();
}

class _AuthScreenState extends State<AuthScreen>
    with SingleTickerProviderStateMixin {
  final _formKeyRegister = GlobalKey<FormState>();
  final _formKeyLogin = GlobalKey<FormState>();

  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _otpController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _phoneController = TextEditingController();

  bool _isCodeSent = false;
  bool _isLoading = false;
  bool _isPasswordVisible = false;
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  void _showSnackBar(String message) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor:
            Theme.of(context).primaryColor, // Use primaryColor from theme
      ),
    );
  }

  Widget _buildTextField({
    required String labelText,
    required IconData icon,
    required String? Function(String?) validator,
    required TextEditingController controller,
    bool obscureText = false,
    VoidCallback? toggleVisibility,
  }) {
    return TextFormField(
      controller: controller,
      obscureText: obscureText,
      validator: validator,
      style: TextStyle(
          fontSize: 16,
          color: Theme.of(context).primaryColor), // Use primaryColor
      decoration: InputDecoration(
        hintText: labelText,
        hintStyle: TextStyle(
            color: Theme.of(context)
                .primaryColor
                .withOpacity(0.6)), // Use primaryColor
        focusedBorder: OutlineInputBorder(
          borderSide: BorderSide(
              color: Theme.of(context).primaryColor), // Use primaryColor
        ),
        enabledBorder: OutlineInputBorder(
          borderSide: BorderSide(
              color: Theme.of(context).primaryColor), // Use primaryColor
        ),
        prefixIcon: Icon(icon,
            color: Theme.of(context).primaryColor), // Use primaryColor
        suffixIcon: toggleVisibility != null
            ? IconButton(
                icon: Icon(
                  obscureText ? Icons.visibility_off : Icons.visibility,
                  color: Theme.of(context).primaryColor, // Use primaryColor
                ),
                onPressed: toggleVisibility,
              )
            : null,
        filled: true,
        fillColor: Colors.white, // White background for fields
      ),
    );
  }

  Widget _buildActionButton(String label, VoidCallback onPressed) {
    return ElevatedButton(
      style: ElevatedButton.styleFrom(
        backgroundColor: Theme.of(context).primaryColor, // Use primaryColor
        foregroundColor: Colors.white,
        shadowColor:
            Theme.of(context).primaryColor.withOpacity(0.5), // Use primaryColor
        minimumSize: Size(double.infinity, 50),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        elevation: 5,
      ),
      onPressed: onPressed,
      child: Text(label, style: TextStyle(fontSize: 16)),
    );
  }

  Widget _buildRegisterTab() {
    return Form(
      key: _formKeyRegister,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          children: [
            _buildTextField(
              labelText: 'Email',
              icon: Icons.email_outlined,
              controller: _emailController,
              validator: (value) {
                if (value == null || value.isEmpty) {
                  return 'Введите email';
                }
                if (!RegExp(r'^[^@]+@[^@]+\.[^@]+').hasMatch(value)) {
                  return 'Введите корректный email';
                }
                return null;
              },
            ),
            const SizedBox(height: 20),
            if (_isCodeSent)
              _buildTextField(
                labelText: 'Введите OTP код',
                icon: Icons.sms_outlined,
                controller: _otpController,
                validator: (value) =>
                    value != null && value.isEmpty ? 'Введите код' : null,
              ),
            if (!_isCodeSent)
              _buildActionButton(
                "Отправить OTP код",
                () async {
                  if (_formKeyRegister.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      await Provider.of<AuthProvider>(context, listen: false)
                          .sendOtp(_emailController.text.trim());
                      setState(() => _isCodeSent = true);
                      _showSnackBar("OTP код отправлен на ваш email");
                    } catch (e) {
                      _showSnackBar("Ошибка: ${e.toString()}");
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
              ),
            if (_isCodeSent)
              _buildTextField(
                labelText: 'Номер телефона', // New phone number input field
                icon: Icons.phone,
                controller: _phoneController,
                validator: (value) {
                  if (value == null || value.isEmpty) {
                    return 'Введите номер телефона';
                  }
                  return null;
                },
              ),
            if (_isCodeSent)
              _buildTextField(
                labelText: 'Пароль',
                icon: Icons.lock,
                controller: _passwordController,
                obscureText: !_isPasswordVisible,
                toggleVisibility: () {
                  setState(() => _isPasswordVisible = !_isPasswordVisible);
                },
                validator: (value) {
                  if (value == null || value.isEmpty) {
                    return 'Введите пароль';
                  }
                  if (value.length < 6) {
                    return 'Пароль должен быть не менее 6 символов';
                  }
                  return null;
                },
              ),
            if (_isCodeSent)
              _buildActionButton(
                "Завершить регистрацию",
                () async {
                  if (_formKeyRegister.currentState?.validate() ?? false) {
                    setState(() => _isLoading = true);
                    try {
                      await Provider.of<AuthProvider>(context, listen: false)
                          .verifyOtp(_emailController.text.trim(),
                              _otpController.text.trim());
                      await Provider.of<AuthProvider>(context, listen: false)
                          .completeRegistration(
                        _emailController.text.trim(),
                        _passwordController.text.trim(),
                        _phoneController.text.trim(),
                      );
                      Navigator.pushReplacementNamed(context, '/main');
                    } catch (e) {
                      _showSnackBar("Ошибка: ${e.toString()}");
                    } finally {
                      setState(() => _isLoading = false);
                    }
                  }
                },
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildLoginTab() {
    return Form(
      key: _formKeyLogin,
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          children: [
            _buildTextField(
              labelText: 'Email',
              icon: Icons.email,
              controller: _emailController,
              validator: (value) {
                if (value == null || value.isEmpty) {
                  return 'Введите email';
                }
                if (!RegExp(r'^[^@]+@[^@]+\.[^@]+').hasMatch(value)) {
                  return 'Введите корректный email';
                }
                return null;
              },
            ),
            const SizedBox(height: 20),
            _buildTextField(
              labelText: 'Пароль',
              icon: Icons.lock,
              controller: _passwordController,
              obscureText: !_isPasswordVisible,
              toggleVisibility: () {
                setState(() => _isPasswordVisible = !_isPasswordVisible);
              },
              validator: (value) {
                if (value == null || value.isEmpty) {
                  return 'Введите пароль';
                }
                if (value.length < 6) {
                  return 'Пароль должен быть не менее 6 символов';
                }
                return null;
              },
            ),
            const SizedBox(height: 20),
            _buildActionButton(
              "Войти",
              () async {
                if (_formKeyLogin.currentState?.validate() ?? false) {
                  setState(() => _isLoading = true);
                  try {
                    await Provider.of<AuthProvider>(context, listen: false)
                        .login(_emailController.text.trim(),
                            _passwordController.text.trim(), context);
                    Navigator.pushReplacementNamed(context, '/main');
                  } catch (e) {
                    _showSnackBar("Ошибка: ${e.toString()}");
                  } finally {
                    setState(() => _isLoading = false);
                  }
                }
              },
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
            color: Colors.white, // White background
          ),
          Align(
            alignment: Alignment(0, 0.5), // Смещение вниз по оси Y
            child: Container(
              width: 180,
              height: 180,
              decoration: BoxDecoration(
                color: Colors.white, // White circle background
                shape: BoxShape.circle,
              ),
              child: Padding(
                padding: const EdgeInsets.all(15.0),
                child: Image.asset(
                  'assets/amanzat_logo.png', // Logo image
                  width: 400,
                  height: 400,
                ),
              ),
            ),
          ),
          Column(
            children: [
              const SizedBox(height: 80),
              Animate(
                effects: [FadeEffect(duration: 1.seconds)],
                child: Text(
                  "Добро пожаловать!",
                  style: TextStyle(
                    fontSize: 26,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).primaryColor, // Use primaryColor
                  ),
                ),
              ),
              const SizedBox(height: 20),
              Animate(
                effects: [
                  SlideEffect(
                    begin: Offset(0, -1),
                    end: Offset(0, 0),
                    duration: 1.seconds,
                  ),
                ],
                child: TabBar(
                  controller: _tabController,
                  indicatorColor:
                      Theme.of(context).primaryColor, // Use primaryColor
                  labelColor:
                      Theme.of(context).primaryColor, // Use primaryColor
                  unselectedLabelColor: Theme.of(context)
                      .primaryColor
                      .withOpacity(0.6), // Lighter green for unselected tabs
                  tabs: const [
                    Tab(text: 'Регистрация'),
                    Tab(text: 'Вход'),
                  ],
                ),
              ),
              Expanded(
                child: TabBarView(
                  controller: _tabController,
                  children: [
                    _buildRegisterTab(),
                    _buildLoginTab(),
                  ],
                ),
              ),
            ],
          ),
          if (_isLoading)
            Center(
              child: Lottie.asset(
                'assets/animation/loading_animation.json',
                width: 100,
                height: 100,
                fit: BoxFit.cover,
              ),
            ),
        ],
      ),
    );
  }
}
