import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class ProfileScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final authProvider = Provider.of<AuthProvider>(context);
    final currentUser = authProvider.currentUser;
    final login = authProvider.currentUser['login'];

    return Scaffold(
      appBar: AppBar(
        backgroundColor: Color(0xFF6A0DAD), // основной цвет
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
      body: currentUser == null
          ? Center(
              child: Text(
                'Вы не авторизованы. Войдите в аккаунт.',
                style: TextStyle(fontSize: 16, color: Colors.grey),
              ),
            )
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16.0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  CircleAvatar(
                    radius: 50,
                    backgroundColor: Colors.green.shade100,
                    backgroundImage: currentUser['avatarUrl'] != null
                        ? NetworkImage(currentUser['avatarUrl'])
                        : null,
                    child: currentUser['avatarUrl'] == null
                        ? const Icon(Icons.person,
                            size: 50, color: Colors.white)
                        : null,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    login ?? 'Логин не указан',
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
                  Text(
                    currentUser['email'] ?? 'Email не указан',
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
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Политика конфиденциальности',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () {
                      Navigator.pushNamed(context, '/privacy_policy');
                    },
                  ),
                  const Divider(),
                  ListTile(
                    leading: Icon(Icons.gavel_outlined,
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Пользовательское соглашение',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () {
                      Navigator.pushNamed(context, '/user_agreement');
                    },
                  ),
                  const Divider(),
                  ListTile(
                    leading: Icon(Icons.support_agent,
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Связаться с нами',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
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
                      backgroundColor: Color(0xFF6A0DAD), // основной цвет
                      padding: const EdgeInsets.symmetric(
                          horizontal: 30, vertical: 15),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10),
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}
