import env from './env'
import axios from 'axios'

export default () => {

    const userRoleSelectInputs = document.querySelectorAll('[data-user-role-select]')


    userRoleSelectInputs.forEach(roleSelect => {
        roleSelect.addEventListener('change', evt => {

            const user_id = roleSelect.getAttribute('data-user-id')
            const role_id = evt.currentTarget.value

            axios.post(env.APP_URL + "/admin/setUserRole", {user_id, role_id})
                .then(r => {
                    alert("Role Set")
                })
                .catch(e => {
                    console.error(e)
                    alert("Could not set user role.")
                })
        })
    })
}
