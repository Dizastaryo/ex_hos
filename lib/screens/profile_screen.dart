import 'package:flutter/material.dart';
import '../services/category_service.dart'; // импортируем сервис
import '../models/profile.dart'; // импортируем модель профиля
import 'package:dio/dio.dart'; // для создания Dio
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart'; // всё ещё нужен для logout

class ProfileScreen extends StatefulWidget {
  @override
  _ProfileScreenState createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  late Future<Profile> _profileFuture;
  final CategoryService _categoryService =
      CategoryService(Dio()); // создаём Dio внутри экрана

  @override
  void initState() {
    super.initState();
    _profileFuture =
        _categoryService.getProfile(); // запускаем загрузку профиля
  }

  @override
  Widget build(BuildContext context) {
    final authProvider = Provider.of<AuthProvider>(context, listen: false);

    return Scaffold(
      appBar: AppBar(
        backgroundColor: Color(0xFF6A0DAD),
        elevation: 0,
        centerTitle: true,
        title: const Text(
          'Профиль',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 22,
            color: Colors.white,
            letterSpacing: 1.2,
          ),
        ),
      ),
      body: FutureBuilder<Profile>(
        future: _profileFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          } else if (snapshot.hasError) {
            return Center(
              child: Text(
                'Ошибка загрузки профиля',
                style: TextStyle(fontSize: 16, color: Colors.red),
              ),
            );
          } else if (!snapshot.hasData) {
            return Center(
              child: Text(
                'Профиль не найден',
                style: TextStyle(fontSize: 16, color: Colors.grey),
              ),
            );
          }

          final profile = snapshot.data!;

          return SingleChildScrollView(
            padding: const EdgeInsets.all(16.0),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                CircleAvatar(
                  radius: 50,
                  backgroundColor: Colors.green.shade100,
                  child: Icon(Icons.person, size: 50, color: Colors.white),
                ),
                const SizedBox(height: 16),
                Text(
                  profile.username, // <<< Теперь тут имя пользователя
                  style: const TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.bold,
                    color: Colors.black,
                    shadows: [
                      Shadow(
                        color: Colors.black26,
                        offset: Offset(1, 1),
                        blurRadius: 2,
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 20),
                ListTile(
                  leading: Icon(Icons.privacy_tip_outlined,
                      color: Color(0xFF6A0DAD)),
                  title: Text('Политика конфиденциальности',
                      style: TextStyle(fontSize: 16)),
                  trailing: Icon(Icons.arrow_forward_ios, size: 16),
                  onTap: () {
                    Navigator.pushNamed(context, '/privacy_policy');
                  },
                ),
                const Divider(),
                ListTile(
                  leading: Icon(Icons.gavel_outlined, color: Color(0xFF6A0DAD)),
                  title: Text('Пользовательское соглашение',
                      style: TextStyle(fontSize: 16)),
                  trailing: Icon(Icons.arrow_forward_ios, size: 16),
                  onTap: () {
                    Navigator.pushNamed(context, '/user_agreement');
                  },
                ),
                const Divider(),
                ListTile(
                  leading: Icon(Icons.support_agent, color: Color(0xFF6A0DAD)),
                  title:
                      Text('Связаться с нами', style: TextStyle(fontSize: 16)),
                  trailing: Icon(Icons.arrow_forward_ios, size: 16),
                  onTap: () {
                    Navigator.pushNamed(context, '/support');
                  },
                ),
                const Divider(),
                const SizedBox(height: 20),
                ElevatedButton.icon(
                  onPressed: () async {
                    await authProvider.logout(context);
                    Navigator.pushReplacementNamed(context, '/auth');
                  },
                  icon: const Icon(Icons.logout, color: Colors.white),
                  label: const Text(
                    'Выйти из аккаунта',
                    style: TextStyle(color: Colors.white),
                  ),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Color(0xFF6A0DAD),
                    padding: const EdgeInsets.symmetric(
                        horizontal: 30, vertical: 15),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10),
                    ),
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
