import env from './env'
import axios from 'axios'

export default () => {
    const trigger = document.querySelector('[data-story-delete]')

    const $modal = $('[data-issuez-delete-modal="story"]')

    let targetStory = null
    let deleteConfirmBtn = null

    $modal.on('hidden.bs.modal', function (e) {
        deleteConfirmBtn.removeEventListener('click', confirmDelete)
    })

    $modal.on('show.bs.modal', function (e) {
        deleteConfirmBtn = document.querySelector('[data-story-delete-confirm]')

        const content = document.querySelector('[data-delete-modal-content]')
        const title = document.querySelector('[data-delete-modal-title]')

        title.innerHTML = "Delete " + (targetStory.Name || '')

        content.innerHTML = 'Are you sure?'

        deleteConfirmBtn.addEventListener('click', confirmDelete)
    })

    trigger.addEventListener('click', evt => {

        const storyDataRaw = trigger.getAttribute('data-story-delete')

        targetStory = storyDataRaw
            ? JSON.parse(storyDataRaw)
            : {}

        $modal.modal('show')
    })

    function confirmDelete() {

        axios.delete(`${env.APP_URL}/stories/${targetStory.ID}`)
            .then(resp => {
                window.location.href = `${env.APP_URL}/features/${targetStory.FeatureID}`
            })
            .catch(err => {
                alert(err.message)
                console.log(err)
            })
    }
}

